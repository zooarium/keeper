package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GuestKey holds the schema definition for the GuestKey entity. A guest key
// is a publishable site key (Stripe-publishable-key style): public services
// exchange it for a short-lived, tenant-scoped guest JWT via
// POST /guest-keys/auth. The referenced user is the designated guest
// identity all unauthenticated traffic for that tenant runs under.
type GuestKey struct {
	ent.Schema
}

// Annotations of the GuestKey.
func (GuestKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "kpr_guest_key"},
	}
}

// Fields of the GuestKey.
func (GuestKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("division_id"),
		field.Int("user_id"),
		field.String("name"),
		field.String("site_key").Unique(),
		// domain is the normalized URL (host[+path], scheme/port stripped,
		// lowercased host) the publishable UI is served from. Unique so a
		// public lookup by URL resolves to exactly one site key.
		field.String("domain").Unique(),
		field.Int8("status").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Indexes of the GuestKey.
func (GuestKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id"),
	}
}
