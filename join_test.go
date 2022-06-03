package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

type JoinTestModel struct {
	Relation *JoinRelationModel
	Name     string
	ID       int `gorm:"primaryKey"`
	RelID    int `gorm:"column:relation_id"`
}

func (m *JoinTestModel) TableName() string {
	return "table"
}

type JoinRelationModel struct {
	B string
	A int `gorm:"primaryKey"`
}

func (m *JoinRelationModel) TableName() string {
	return "relation"
}

func TestJoinScope(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "notarelation", Fields: []string{"a", "b", "notacolumn"}}
	join.selectCache = map[string][]string{}

	schema, err := parseModel(db, &JoinTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, join.Scopes(&Settings{}, schema))
	join.Relation = "Relation"

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(&Settings{}, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`a`", "`relation`.`b`"}, tx.Statement.Selects)
	}
	assert.Equal(t, []string{"a", "b", "notacolumn"}, join.selectCache["Relation"])
}

func TestJoinScopeAnonymousRelation(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "notarelation", Fields: []string{"a", "b", "notacolumn"}}
	join.selectCache = map[string][]string{}

	type JoinTestModel struct {
		Relation *struct {
			B string
			A int `gorm:"primaryKey"`
		}
		Name  string
		ID    int `gorm:"primaryKey"`
		RelID int `gorm:"column:relation_id"`
	}

	schema, err := parseModel(db, &JoinTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, join.Scopes(&Settings{}, schema))
	join.Relation = "Relation"

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(&Settings{}, schema)...).Table("table").Find(&results)
	assert.Empty(t, db.Statement.Preloads)
	assert.Empty(t, db.Statement.Selects)
	assert.Equal(t, "Relation \"Relation\" is anonymous, could not get table name", db.Error.Error())
	assert.Equal(t, []string{"a", "b", "notacolumn"}, join.selectCache["Relation"])
}

func TestJoinScopeBlacklisted(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation", Fields: []string{"a", "b", "notacolumn"}}

	schema, err := parseModel(db, &JoinTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, join.Scopes(&Settings{Blacklist: Blacklist{RelationsBlacklist: []string{"Relation"}}}, schema))
}

type JoinHopTestModel struct {
	Relation *JoinHopTestChildModel
	Name     string
	ID       int `gorm:"primaryKey"`
	RelID    int `gorm:"column:relation_id"`
}

func (m *JoinHopTestChildModel) TableName() string {
	return "relation"
}

type JoinHopTestChildModel struct {
	Parent   *JoinHopTestModel
	B        string
	A        int `gorm:"primaryKey"`
	ParentID int
}

type JoinHopManyTestModel struct {
	Name     string
	Relation []*JoinHopManyTestChildModel `gorm:"foreignKey:A"`
	ID       int                          `gorm:"primaryKey"`
}

type JoinHopManyTestChildModel struct {
	Parent   *JoinHopManyTestModel
	B        string
	A        int `gorm:"primaryKey"`
	ParentID int
}

func (m *JoinHopManyTestChildModel) TableName() string {
	return "relation"
}

func TestJoinScopeBlacklistedRelationHop(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation.Parent.Relation", Fields: []string{"name", "id"}}
	join.selectCache = map[string][]string{}

	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}

	settings := &Settings{
		Blacklist: Blacklist{
			Relations: map[string]*Blacklist{
				"Relation": {
					RelationsBlacklist: []string{"Parent"},
				},
			},
		},
	}

	assert.Nil(t, join.Scopes(settings, schema))
}

func TestJoinScopePrimaryKeyNotSelected(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation", Fields: []string{"b"}}
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	schema.Table = "table"

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(&Settings{}, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`b`", "`relation`.`a`", "`relation`.`parent_id`"}, tx.Statement.Selects)
	}
	assert.Equal(t, []string{"b"}, join.selectCache["Relation"])

	// Don't select it if it's blacklisted
	settings := &Settings{
		Blacklist: Blacklist{
			Relations: map[string]*Blacklist{
				"Relation": {
					FieldsBlacklist: []string{"a"},
				},
			},
		},
	}
	db = db.Scopes(join.Scopes(settings, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`b`", "`relation`.`parent_id`"}, tx.Statement.Selects)
	}
}

func TestJoinScopeHasMany(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation", Fields: []string{"a", "b"}}
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	schema.Table = "table"

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(&Settings{}, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`a`", "`relation`.`b`", "`relation`.`parent_id`"}, tx.Statement.Selects)
	}
	assert.Equal(t, []string{"a", "b"}, join.selectCache["Relation"])

	// Don't select parent_id if blacklisted
	settings := &Settings{
		Blacklist: Blacklist{
			Relations: map[string]*Blacklist{
				"Relation": {
					FieldsBlacklist: []string{"parent_id"},
				},
			},
		},
	}
	db = db.Scopes(join.Scopes(settings, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`a`", "`relation`.`b`"}, tx.Statement.Selects)
	}
}

func TestJoinScopeNestedRelations(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation.Parent", Fields: []string{"id", "relation_id"}}
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}

	settings := &Settings{
		Blacklist: Blacklist{
			FieldsBlacklist: []string{"name"},
			Relations: map[string]*Blacklist{
				"Relation": {
					FieldsBlacklist: []string{"b"},
					Relations: map[string]*Blacklist{
						"Parent": {
							FieldsBlacklist: []string{"relation_id"},
							IsFinal:         true,
						},
					},
				},
			},
		},
	}

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(settings, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation.Parent") {
		tx := db.Session(&gorm.Session{}).Scopes(db.Statement.Preloads["Relation.Parent"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`table`.`id`"}, tx.Statement.Selects)
	}
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Session(&gorm.Session{}).Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`a`", "`relation`.`parent_id`"}, tx.Statement.Selects)
	}
	assert.NotContains(t, join.selectCache, "Relation")
	assert.Equal(t, []string{"id", "relation_id"}, join.selectCache["Relation.Parent"])
}

func TestJoinScopeFinal(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation", Fields: []string{"a", "b"}}
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	settings := &Settings{Blacklist: Blacklist{IsFinal: true}}

	assert.Nil(t, join.Scopes(settings, schema))
}

func TestJoinNestedRelationsWithSelect(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation", Fields: []string{"b"}}
	join.selectCache = map[string][]string{}
	join2 := &Join{Relation: "Relation.Parent", Fields: []string{"id", "relation_id"}}
	join2.selectCache = join.selectCache
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	settings := &Settings{
		Blacklist: Blacklist{
			FieldsBlacklist: []string{"name"},
			Relations: map[string]*Blacklist{
				"Relation": {
					Relations: map[string]*Blacklist{
						"Parent": {
							FieldsBlacklist: []string{"relation_id"},
							IsFinal:         true,
						},
					},
				},
			},
		},
	}

	results := map[string]interface{}{}
	db = db.Scopes(join.Scopes(settings, schema)...).Scopes(join2.Scopes(settings, schema)...).Table("table").Find(&results)
	if assert.Contains(t, db.Statement.Preloads, "Relation.Parent") {
		tx := db.Session(&gorm.Session{}).Scopes(db.Statement.Preloads["Relation.Parent"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`table`.`id`"}, tx.Statement.Selects)
	}
	if assert.Contains(t, db.Statement.Preloads, "Relation") {
		tx := db.Session(&gorm.Session{}).Scopes(db.Statement.Preloads["Relation"][0].(func(*gorm.DB) *gorm.DB)).Find(nil)
		assert.Equal(t, []string{"`relation`.`b`", "`relation`.`a`", "`relation`.`parent_id`"}, tx.Statement.Selects)
	}
	assert.Equal(t, []string{"b"}, join.selectCache["Relation"])
	assert.Equal(t, []string{"id", "relation_id"}, join.selectCache["Relation.Parent"])
}

func TestJoinScopeInvalidSyntax(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation.", Fields: []string{"a", "b"}} // A dot at the end of the relation name is invalid
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, join.Scopes(&Settings{}, schema))
}

func TestJoinScopeNonExistingRelation(t *testing.T) {
	db, _ := gorm.Open(&tests.DummyDialector{}, nil)
	join := &Join{Relation: "Relation.NotARelation.Parent", Fields: []string{"a", "b"}}
	join.selectCache = map[string][]string{}
	schema, err := parseModel(db, &JoinHopManyTestModel{})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, join.Scopes(&Settings{}, schema))
}
