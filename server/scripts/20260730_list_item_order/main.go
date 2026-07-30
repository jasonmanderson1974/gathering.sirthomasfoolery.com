// Backfills EventListItem.order for lists written before F17 added it.
//
// WHY IT IS NEEDED. Ordering is fractional: dropping an entry between two
// neighbours stores the midpoint of their orders. Entries written before F17
// have no order field at all, which decodes to 0 — so a whole legacy list is
// one big tie. Display breaks that tie by array position, which is why the app
// is correct on unmigrated data and why this script is not urgent. But a drop
// BETWEEN two tied entries computes (0+0)/2 = 0, landing the moved entry in the
// tie it was trying to escape, and since a cross-list move re-appends the entry
// at the end of the array it then renders last. The drop looks like it was
// ignored. Spreading legacy entries out over real orders is what gives a drop
// somewhere to land.
//
// WHAT IT DOES. For every event that has lists, each list holding AT LEAST ONE
// entry with no order field is renumbered (i+1)*1024 by its current array
// order — the order those entries are displayed in today, so nothing appears to
// move. Lists whose entries all carry an order are left alone.
//
// RERUN SAFETY. Safe to re-run: after a successful pass every entry has an
// order, so no list matches the "at least one missing" test and the script
// makes no writes. It is also safe to run against a database that has been
// live on F17 for a while — a list where someone has already dragged an entry
// has orders on every entry (adds stamp one) and is skipped, so a person's
// arrangement is never flattened back to insertion order.
//
// This one reads-modify-writes whole lists, which the runtime code goes out of
// its way never to do. That is fine here and only here: it runs once, offline,
// against a database nobody is dragging entries in at the time.
//
// USAGE (dry run first — it prints what it would touch and writes nothing):
//
//	go run ./scripts/20260730_list_item_order -dry-run
//	MONGODB_URI=mongodb://localhost:27017 go run ./scripts/20260730_list_item_order
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Mirrors listItemOrderStep in routes/event_lists.go. Duplicated rather than
// imported because these scripts are deliberately standalone and frozen — see
// scripts/README.md.
const orderStep = 1024

func main() {
	dryRun := flag.Bool("dry-run", false, "report what would change without writing")
	flag.Parse()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	events := client.Database("schej-it").Collection("events")

	// Only events that actually have a list. `lists.0` existing is the cheap
	// way to say "a non-empty array".
	cursor, err := events.Find(context.Background(), bson.M{"lists.0": bson.M{"$exists": true}})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.Background())

	var scanned, touchedEvents, touchedLists, touchedItems int

	for cursor.Next(context.Background()) {
		// Decoded as raw bson, NOT into models.Event: the whole test is whether
		// the order field is ABSENT, and decoding into a struct turns absent
		// into 0, which is indistinguishable from a stored zero.
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("skipping an event that would not decode: %v", err)
			continue
		}
		scanned++

		eventId, ok := doc["_id"].(primitive.ObjectID)
		if !ok {
			log.Printf("skipping an event with no usable _id")
			continue
		}
		lists, ok := doc["lists"].(primitive.A)
		if !ok {
			continue
		}

		set := bson.M{}
		listsInThisEvent := 0

		for listIdx, rawList := range lists {
			list, ok := rawList.(bson.M)
			if !ok {
				continue
			}
			items, ok := list["items"].(primitive.A)
			if !ok || len(items) == 0 {
				continue
			}

			needsWork := false
			for _, rawItem := range items {
				if item, ok := rawItem.(bson.M); ok {
					if _, present := item["order"]; !present {
						needsWork = true
						break
					}
				}
			}
			if !needsWork {
				continue
			}

			// Renumber the WHOLE list, not just the entries that are missing a
			// value. A list part-way through — say someone dragged one entry
			// before this ran — would otherwise end up with a real order sitting
			// among freshly assigned ones that mean something different.
			for itemIdx := range items {
				path := fmt.Sprintf("lists.%d.items.%d.order", listIdx, itemIdx)
				set[path] = float64((itemIdx + 1) * orderStep)
			}
			listsInThisEvent++
			touchedItems += len(items)
		}

		if len(set) == 0 {
			continue
		}
		touchedEvents++
		touchedLists += listsInThisEvent

		if *dryRun {
			fmt.Printf("would renumber %d list(s), %d entries on event %s\n",
				listsInThisEvent, len(set), eventId.Hex())
			continue
		}

		// One update per event, so an event's lists are renumbered together or
		// not at all.
		if _, err := events.UpdateByID(context.Background(), eventId, bson.M{"$set": set}); err != nil {
			log.Printf("event %s: %v", eventId.Hex(), err)
			continue
		}
		fmt.Printf("renumbered %d list(s), %d entries on event %s\n",
			listsInThisEvent, len(set), eventId.Hex())
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	verb := "renumbered"
	if *dryRun {
		verb = "would renumber (dry run, nothing written)"
	}
	fmt.Printf("\nScanned %d event(s) with lists. %s %d list(s) / %d entries across %d event(s).\n",
		scanned, verb, touchedLists, touchedItems, touchedEvents)
}
