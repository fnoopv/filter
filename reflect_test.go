package filter

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils/tests"
)

type TestRelation struct {
	TestModel          *TestModel `gorm:"foreignKey:TestModelID"`
	TestModelGuessed   *TestModel
	Name               string
	TestModelID        int
	TestModelGuessedID int
	ID                 int
}
type Promoted struct {
	Email string `gorm:"column:email_address"`
}
type PromotedPtr struct {
	Promoted
}
type PromotedRelation struct {
	PromotedRelation TestRelation
}
type TestModel struct {
	Relations []*TestRelation
	Relation  *TestRelation
	DeletedAt *gorm.DeletedAt
	*PromotedPtr
	Promoted
	Str string `gorm:"column:"`
	PromotedRelation
	ID uint `gorm:"primaryKey"`
}

func TestParseModel(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	identity := parseModel(db, &TestModel{})

	relModelIdentity := &modelIdentity{
		Columns: map[string]column{
			"name":                  {Name: "Name", Tags: &gormTags{}},
			"id":                    {Name: "ID", Tags: &gormTags{}},
			"test_model_id":         {Name: "TestModelID", Tags: &gormTags{}},
			"test_model_guessed_id": {Name: "TestModelGuessedID", Tags: &gormTags{}},
		},
		Relations: map[string]*relation{},
	}
	expected := &modelIdentity{
		Columns: map[string]column{
			"id":            {Name: "ID", Tags: &gormTags{PrimaryKey: true}},
			"str":           {Name: "Str", Tags: &gormTags{}},
			"email_address": {Name: "Email", Tags: &gormTags{Column: "email_address"}},
			"deleted_at":    {Name: "DeletedAt", Tags: &gormTags{}},
		},
		Relations: map[string]*relation{
			"Relation": {
				modelIdentity: relModelIdentity,
				Type:          schema.HasOne,
				Tags:          &gormTags{},
				PrimaryKeys:   []string{"id"},
				ForeignKeys:   []string{"test_model_id", "test_model_guessed_id"},
				keysProcessed: true,
			},
			"Relations": {
				modelIdentity: relModelIdentity,
				Type:          schema.HasMany,
				Tags:          &gormTags{},
				PrimaryKeys:   []string{"id"},
				ForeignKeys:   []string{"test_model_id", "test_model_guessed_id"},
				keysProcessed: true,
			},
			"PromotedRelation": {
				modelIdentity: relModelIdentity,
				Type:          schema.HasOne,
				Tags:          &gormTags{},
				PrimaryKeys:   []string{"id"},
				ForeignKeys:   []string{"test_model_id", "test_model_guessed_id"},
				keysProcessed: true,
			},
		},
	}
	relModelIdentity.Relations["TestModel"] = &relation{
		modelIdentity: expected,
		Type:          schema.HasOne,
		Tags:          &gormTags{ForeignKey: "TestModelID"},
		PrimaryKeys:   []string{"id"},
		ForeignKeys:   []string{},
		keysProcessed: true,
	}
	relModelIdentity.Relations["TestModelGuessed"] = &relation{
		modelIdentity: expected,
		Type:          schema.HasOne,
		Tags:          &gormTags{},
		PrimaryKeys:   []string{"id"},
		ForeignKeys:   []string{},
		keysProcessed: true,
	}
	assertModelIdentityEqual(t, expected, identity, []*modelIdentity{})

	assert.Same(t, identity.Relations["Relation"].modelIdentity, identity.Relations["Relations"].modelIdentity)
	assert.Same(t, identity.Relations["Relation"].modelIdentity, identity.Relations["PromotedRelation"].modelIdentity)

	assert.Contains(t, identityCache, "goyave.dev/filter|filter.TestRelation")
	assert.Contains(t, identityCache, "goyave.dev/filter|filter.Promoted")
	assert.Contains(t, identityCache, "goyave.dev/filter|filter.PromotedPtr")
	assert.Contains(t, identityCache, "goyave.dev/filter|filter.PromotedRelation")
	assert.Contains(t, identityCache, "goyave.dev/filter|filter.TestModel")

	identity = parseModel(db, []*TestModel{})
	assert.Equal(t, expected, identity)
}

func assertModelIdentityEqual(t *testing.T, expected *modelIdentity, actual *modelIdentity, explored []*modelIdentity) {
	assert.Equal(t, expected.Columns, actual.Columns)
	for k, v := range expected.Relations {
		if assert.Contains(t, actual.Relations, k) {
			v2 := actual.Relations[k]
			explored = append(explored, v.modelIdentity)
			if !isExplored(explored, v.modelIdentity) {
				assertModelIdentityEqual(t, v.modelIdentity, v2.modelIdentity, explored)
			}
			assert.Equal(t, v.Type, v2.Type)
			assert.Equal(t, v.Tags, v2.Tags)
			assert.Equal(t, v.keysProcessed, v2.keysProcessed)
			assert.ElementsMatch(t, v.PrimaryKeys, v2.PrimaryKeys)
			assert.ElementsMatch(t, v.ForeignKeys, v2.ForeignKeys)
		}
	}
	for k := range actual.Relations {
		assert.Contains(t, expected.Relations, k)
	}
	assert.Equal(t, expected.Columns, actual.Columns)
}

func isExplored(explored []*modelIdentity, identity *modelIdentity) bool {
	for _, v := range explored {
		if v == identity {
			return true
		}
	}
	return false
}

type TestRelationCycle struct {
	Parent *TestModelRelationCycle
}
type TestModelRelationCycle struct {
	*TestModelRelationCycle
	Relation *TestRelationCycle
}

func TestParseModelRelationCycle(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	identity := parseModel(db, &TestModelRelationCycle{})

	rel := &relation{
		modelIdentity: &modelIdentity{
			Columns:   map[string]column{},
			Relations: map[string]*relation{},
		},
		Type:          schema.HasOne,
		Tags:          &gormTags{},
		PrimaryKeys:   []string{},
		ForeignKeys:   []string{},
		keysProcessed: true,
	}
	expected := &modelIdentity{
		Columns: map[string]column{},
		Relations: map[string]*relation{
			"Relation": rel,
		},
	}
	rel.Relations["Parent"] = &relation{
		modelIdentity: expected,
		Type:          schema.HasOne,
		Tags:          &gormTags{},
		PrimaryKeys:   []string{},
		ForeignKeys:   []string{},
		keysProcessed: true,
	}
	assert.Equal(t, expected, identity)
}

func TestParseModelEmbeddedStruct(t *testing.T) {
	type TestEmbed struct {
		Name string
	}
	type TestModelEmbedded struct {
		Embed TestEmbed `gorm:"embedded;embeddedPrefix:embed_"`
	}

	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	identity := parseModel(db, &TestModelEmbedded{})
	expected := &modelIdentity{
		Columns: map[string]column{
			"embed_name": {Name: "Name", Tags: &gormTags{}},
		},
		Relations: map[string]*relation{},
	}
	assert.Equal(t, expected, identity)
}

func TestParseGormTags(t *testing.T) {
	type gormTagsModel struct {
		CustomColumn string `gorm:"column:custom_column"`
		Relation     string `gorm:"foreignKey:id_relation;references:relation"`
		Embedded     string `gorm:"embedded;embeddedPrefix:prefix_"`
		ID           int    `gorm:"primaryKey"`
		IDAlt        int    `gorm:"primary_key"`
	}

	ty := reflect.TypeOf(gormTagsModel{})
	expected := &gormTags{Column: "custom_column"}
	assert.Equal(t, expected, parseGormTags(ty.Field(0)))

	expected = &gormTags{ForeignKey: "id_relation", References: "relation"}
	assert.Equal(t, expected, parseGormTags(ty.Field(1)))

	expected = &gormTags{Embedded: true, EmbeddedPrefix: "prefix_"}
	assert.Equal(t, expected, parseGormTags(ty.Field(2)))

	expected = &gormTags{PrimaryKey: true}
	assert.Equal(t, expected, parseGormTags(ty.Field(3)))

	expected = &gormTags{PrimaryKey: true}
	assert.Equal(t, expected, parseGormTags(ty.Field(4)))
}

func TestCleanColumns(t *testing.T) {
	id := &modelIdentity{
		Columns: map[string]column{
			"id":   {},
			"name": {},
		},
	}
	assert.Equal(t, []string{"id", "name"}, id.cleanColumns([]string{"id", "test", "name", "notacolumn"}))
}

func TestParseNilModel(t *testing.T) {
	assert.Nil(t, parseModel(nil, 1))
}
