package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Annotations of the User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "kpr_user"},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("division_id"),
		field.String("firstname"),
		field.String("lastname"),
		field.String("email").Unique(),
		field.String("password").Sensitive(),
		field.Int8("role").Default(0),
		field.Int8("status").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("app", App.Type).
			Ref("users").
			Unique().
			Required().
			Field("app_id").
			Annotations(
				entsql.OnDelete(entsql.Cascade),
			),
		edge.From("division", Division.Type).
			Ref("users").
			Unique().
			Required().
			Field("division_id").
			Annotations(
				entsql.OnDelete(entsql.Restrict),
			),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id"),
	}
}
