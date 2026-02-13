package main

import "math/rand"

var quoteIdx = rand.Intn(len(devQuotes))
var devQuotes = []string{
	"Just use NATS.",
	"Pub/sub all the things!",
	"Messages are the new API.",
	"JetStream for durability.",
	"No more polling.",
	"Event-driven architecture FTW.",
	"Decouple everything.",
	"NATS is fast.",
	"Subjects are like topics.",
	"Request-reply is cool.",
}

func randomDevQuote() string {
	quoteIdx = (quoteIdx + 1) % len(devQuotes)
	return devQuotes[quoteIdx]
}
