// Undo for a cascading assignment (N2).
//
// Assigning a checklist entry assigns everything beneath it, OVERWRITING whoever
// held those sub-entries. N1 shipped that on the argument that it is "exactly
// reversible" — set the parent back to Unassigned. It is not: clearing cascades
// too, so the obvious undo destroys the overwritten assignments rather than
// restoring them. Two clicks and other members' work is gone with no record it
// existed.
//
// So the SERVER records what it replaced, and undo restores that record.
//
// The alternative — the client keeps the snapshot and posts it back — fails on
// our own rule. setEventListItemAssignee validates every assignee against
// assignableMembers, one of whose sources is *anyone already holding an
// assignment*; a cascade takes Bart's entry, so Bart can drop out of the pool,
// and the undo that exists to put him back would be refused. The failure lands
// exactly on the case the feature is for. A client-supplied restore is also
// unverifiable, so the endpoint would have to relax the pool check to "any
// member+" and widen what a crafted request can set. Restoring the server's own
// record has nothing to validate and nothing to forge.
//
// Shape copied deliberately from utils.RateLimiter: mutex + map, lazy expiry on
// read, a ticker janitor, a separately-testable evictStale, and an idempotent
// Stop. Same single-instance assumption, too — the records live in this process
// and are lost on restart, which for a seven-second affordance is not worth a
// collection.
package routes

import (
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

const (
	// undoWindow is how long the server will honour an undo.
	//
	// Deliberately LONGER than the 7 seconds the button is shown for. The two
	// clocks are independent — one in a browser, one here — and the failure that
	// matters is a button that is still on screen when the record behind it has
	// gone. Erring long makes that impossible; erring short makes it routine.
	undoWindow = 30 * time.Second

	undoJanitorInterval = time.Minute
)

// priorAssignment is one entry's assignment as it stood before the write that
// replaced it. A nil AssigneeId means it was unassigned, which is a state worth
// restoring exactly as much as a name is.
type priorAssignment struct {
	ItemId       primitive.ObjectID
	AssigneeId   *primitive.ObjectID
	AssigneeName string
}

// assignUndoRecord is one undoable action.
type assignUndoRecord struct {
	EventId primitive.ObjectID
	ListId  primitive.ObjectID
	Prior   []priorAssignment
	// Token identifies THIS action. Without it a stale button — a second tab, or
	// a second cascade inside the window — would restore a newer action's
	// snapshot, which is a worse outcome than refusing.
	Token string
	At    time.Time
}

// assignUndoStore holds one pending record per user.
//
// Keyed by user id, so two members undo their own actions and never each
// other's. One record per user rather than a stack: "only the most recent action
// is undoable" is then a property of the storage rather than a convention the UI
// has to keep.
type assignUndoStore struct {
	mu       sync.Mutex
	records  map[primitive.ObjectID]assignUndoRecord
	stop     chan struct{}
	stopOnce sync.Once
}

// assignUndos is the process-lifetime singleton, sitting beside its handlers the
// way otpLimiter sits in auth.go.
var assignUndos = newAssignUndoStore()

func newAssignUndoStore() *assignUndoStore {
	s := &assignUndoStore{
		records: make(map[primitive.ObjectID]assignUndoRecord),
		stop:    make(chan struct{}),
	}
	go s.janitor()
	return s
}

// Stop halts the janitor. The singleton never needs it; a store built per test
// would otherwise leak a goroutine and a ticker for the rest of the run. Safe to
// call more than once.
func (s *assignUndoStore) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// Remember stores what an action replaced and returns the token that identifies
// it. Any record the user already had is dropped: the previous action stops
// being undoable the moment a new one lands, which is what stops two live Undo
// buttons disagreeing about what "back" means.
func (s *assignUndoStore) Remember(userId primitive.ObjectID, record assignUndoRecord) string {
	token := primitive.NewObjectID().Hex()

	s.mu.Lock()
	defer s.mu.Unlock()
	record.Token = token
	record.At = time.Now()
	s.records[userId] = record
	return token
}

// Take returns the caller's pending record and removes it, but only on an exact
// match: same user, same gathering, same token, still inside the window.
//
// Removing on success is what makes an undo apply at most once — a double-click
// on the button gets one restore and one refusal, rather than two writes.
func (s *assignUndoStore) Take(
	userId, eventId primitive.ObjectID,
	token string,
) (assignUndoRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, found := s.records[userId]
	if !found {
		return assignUndoRecord{}, false
	}
	// Expiry is checked on read as well as swept by the janitor, so a record can
	// never be honoured late just because the ticker has not come round yet.
	if time.Since(record.At) > undoWindow {
		delete(s.records, userId)
		return assignUndoRecord{}, false
	}
	if record.Token != token || record.EventId != eventId {
		return assignUndoRecord{}, false
	}

	delete(s.records, userId)
	return record, true
}

func (s *assignUndoStore) janitor() {
	ticker := time.NewTicker(undoJanitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.evictStale(undoWindow)
		}
	}
}

// evictStale drops every record older than age. Split out of janitor so it can
// be exercised without waiting on the ticker.
func (s *assignUndoStore) evictStale(age time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-age)
	for userId, record := range s.records {
		if record.At.Before(cutoff) {
			delete(s.records, userId)
		}
	}
}

// snapshotAssignments records the current assignee of each of the given items,
// in the order given, from the list as it was read.
//
// Ids that are no longer on the list are skipped rather than recorded as
// unassigned: restoring them would resurrect an assignment on an entry somebody
// removed in between.
func snapshotAssignments(list *models.EventList, itemIds []primitive.ObjectID) []priorAssignment {
	prior := make([]priorAssignment, 0, len(itemIds))
	for _, id := range itemIds {
		for _, item := range list.Items {
			if item.Id != id {
				continue
			}
			prior = append(prior, priorAssignment{
				ItemId:       item.Id,
				AssigneeId:   item.AssigneeId,
				AssigneeName: item.AssigneeName,
			})
			break
		}
	}
	return prior
}
