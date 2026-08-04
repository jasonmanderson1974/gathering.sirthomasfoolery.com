package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

// Ping asks Mongo to answer, so a caller can tell "the process is up" apart
// from "the process can reach its database".
//
// It runs the command against Db rather than Client on purpose: the mongo
// driver connects lazily and pools connections, so holding a *mongo.Client
// proves nothing about reachability. Going through the database handle also
// proves the named database is addressable — which is what every request that
// matters actually needs — instead of merely that some server answered.
func Ping(ctx context.Context) error {
	return Db.RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
}
