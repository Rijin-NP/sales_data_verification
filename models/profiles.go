package models

import "time"

type Profiles struct {
	ProfileId      string     `json:"profile_id" bson:"profile_id"`
	Geo            string     `json:"geo" bson:"geo"`
	CreatedAt      *time.Time `json:"created_at" bson:"created_at"`
	Email          string     `json:"email" bson:"email"`
	LastAnalyzedAt *time.Time `json:"last_analyzed_at" bson:"last_analyzed_at"`
}

type SellerDaily struct {
	Revenue float64 `json:"revenue" bson:"revenue"`
}

type ProductCount struct {
	Count int64 `json:"product_count" bson:"product_count"`
}
