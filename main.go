package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"bufio"
	"os"
	"os/exec"
	"strings"
)

func main() {
	dbName := "./sql.db"

	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		exec.Command("cp sql.db.init4podcast sql.db")
	}

	db, err := sql.Open("sqite3", dbName)
	dbErr(err)

	inp, err := bufio.NewReader(os.Stdin).ReadString('\n')
	chErr(err)
	newRec := strings.Split(inp, " ")

	query, err := db.Prepare("INSERT INTO podcast (name, url) VALUES(?,?)")
	chErr(err)

	resp, err := query.Exec(newRec[0], newRec[1])
	chErr(err)

	newid, _ := resp.LastInsertId()
	print(newid)
	println("inserted.")

}

func dbErr(err error) {
	if err != nil {
		println("You need initialize sql.db at first.")
		panic(err)
	}
}

func chErr(err error) {
	if err != nil {
		panic(err)
	}
}
