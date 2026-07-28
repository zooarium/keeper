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

// App holds the schema definition for the App entity.
type App struct {
	ent.Schema
}

// Annotations of the App.
func (App) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "kpr_app"},
	}
}

// Fields of the App.
func (App) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Unique(),
		field.String("tagline").Optional(),
		field.String("logo_url").Optional(),
		field.String("about_heading").Optional(),
		field.Text("about_body").Optional(),
		field.String("contact_address_line1").Optional(),
		field.String("contact_address_line2").Optional(),
		field.String("contact_city").Optional(),
		field.String("contact_state").Optional(),
		field.String("contact_country").Optional(),
		field.String("contact_postal_code").Optional(),
		field.String("contact_phone1").Optional(),
		field.String("contact_phone2").Optional(),
		field.String("contact_email").Optional(),
		field.Text("contact_hours").Optional(),
		field.JSON("contact_social", map[string]string{}).Optional(),
		field.String("tax_number").Optional(),
		field.String("currency").NotEmpty().MaxLen(3),
		field.Float("tax_percent").Default(0),
		field.Int("manager_id").Optional().Nillable(),
		field.Int8("status").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the App.
func (App) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type),
		edge.To("divisions", Division.Type),
		edge.To("manager", User.Type).
			Unique().
			Field("manager_id").
			Annotations(
				entsql.OnDelete(entsql.SetNull),
			),
	}
}

// Indexes of the App.
func (App) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("manager_id"),
	}
}
