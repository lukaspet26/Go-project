/*
	SQLite client
	(c) Copyright David Thorpe 2017
	All Rights Reserved

	For Licensing and Usage information, please see LICENSE file
*/

package sqlite

import (
	"regexp"
	"strings"
	"sync"
)

/////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	reserved_words = `ABORT ACTION ADD AFTER ALL ALTER ANALYZE AND 
	AS ASC ATTACH AUTOINCREMENT
	BEFORE BEGIN BETWEEN BY CASCADE CASE CAST CHECK COLLATE COLUMN 
	COMMIT CONFLICT CONSTRAINT CREATE CROSS CURRENT_DATE CURRENT_TIME 
	CURRENT_TIMESTAMP DATABASE DEFAULT DEFERRABLE DEFERRED DELETE 
	DESC DETACH DISTINCT DROP EACH ELSE END ESCAPE EXCEPT EXCLUSIVE 
	EXISTS EXPLAIN FAIL FOR FOREIGN FROM FULL GLOB GROUP HAVING IF
	IGNORE IMMEDIATE IN INDEX INDEXED INITIALLY INNER INSERT INSTEAD
	INTERSECT INTO IS ISNULL JOIN KEY LEFT LIKE LIMIT MATCH NATURAL
	NO NOT NOTNULL NULL OF OFFSET ON OR ORDER OUTER PLAN PRAGMA PRIMARY
	QUERY RAISE RECURSIVE REFERENCES REGEXP REINDEX RELEASE RENAME
	REPLACE RESTRICT RIGHT ROLLBACK ROW SAVEPOINT SELECT SET TABLE
	TEMP TEMPORARY THEN TO TRANSACTION TRIGGER UNION UNIQUE UPDATE
	USING VACUUM VALUES VIEW VIRTUAL WHEN WHERE WITH WITHOUT`
)

/////////////////////////////////////////////////////////////////////
// GLOBAL VARIABLES

var (
	reservedWords        = make(map[string]bool, 0)
	regexpBareIdentifier = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func isBareIdentifier(value string) bool {
	return regexpBareIdentifier.MatchString(value)
}

func isReservedWord(value string) bool {
	var once sync.Once
	once.Do(func() {
		for _, word := range strings.Fields(reserved_words) {
			reservedWords[strings.ToUpper(word)] = true
		}
	})
	value = strings.TrimSpace(strings.ToUpper(value))
	_, exists := reservedWords[value]
	return exists
}

/////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// DoubleQuote puts double quotes around a string
// and escapes existing double quotes
func DoubleQuote(value string) string {
	// Change " into ""
	if strings.Contains(value, "\"") {
		value = strings.Replace(value, "\"", "\"\"", -1)
	}
	return "\"" + value + "\""
}

// QuoteIdentifier returns a safe version of an identifier
func QuoteIdentifier(value string) string {
	if isReservedWord(value) {
		// Check for reserved keyword
		return DoubleQuote(value)
	} else if isBareIdentifier(value) {
		return value
	} else {
		return DoubleQuote(value)
	}
}
