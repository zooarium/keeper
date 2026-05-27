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

// Division holds the schema definition for the Division entity.
type Division struct {
	ent.Schema
}

// Annotations of the Division.
func (Division) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "kpr_division"},
	}
}

// Fields of the Division.
func (Division) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("parent_id").Optional().Nillable(),
		field.String("name"),
		field.String("path"),
		field.Int8("depth").Default(0),
		field.Int8("status").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Division.
func (Division) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Division.Type),
		edge.From("parent", Division.Type).
			Ref("children").
			Unique().
			Field("parent_id"),
		edge.From("app", App.Type).
			Ref("divisions").
			Unique().
			Required().
			Field("app_id").
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("users", User.Type),
	}
}

// Indexes of the Division.
func (Division) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("path"),
		index.Fields("app_id", "parent_id"),
	}
}
