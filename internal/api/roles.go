package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// roleInfo describes how a member role maps onto tables and columns.
type roleInfo struct {
	Table       string
	IDCol       string
	ParentCol   string // "" for regional_coordinator
	ParentTable string
}

var roles = map[string]roleInfo{
	"executive":            {Table: "executive", IDCol: "ex_id", ParentCol: "sv_id", ParentTable: "supervisor"},
	"supervisor":           {Table: "supervisor", IDCol: "sv_id", ParentCol: "dc_id", ParentTable: "district_coordinator"},
	"district_coordinator": {Table: "district_coordinator", IDCol: "dc_id", ParentCol: "rc_id", ParentTable: "regional_coordinator"},
	"regional_coordinator": {Table: "regional_coordinator", IDCol: "rc_id", ParentCol: "", ParentTable: ""},
}

func roleMeta(role string) (roleInfo, bool) {
	ri, ok := roles[role]
	return ri, ok
}

// memberLookup builds the "SELECT name" query for a member.
func memberLookup(role string, id int) (string, []any, error) {
	ri, ok := roleMeta(role)
	if !ok {
		return "", nil, errors.New("unknown role " + role)
	}
	return "SELECT name FROM " + ri.Table + " WHERE " + ri.IDCol + " = $1", []any{id}, nil
}

func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// queryStrings reads a string query parameter with a default.
func queryStrings(r *http.Request, key, def string) string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	return v
}

// nullString converts a nullable string to "".
func nullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
