package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"bufio"
	"os"

	"fmt"
	"os/exec"
	"strings"
	"time"
)

func main() {
	dbName := "./sql.db"

	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		err := exec.Command("cp", "sql.db.init4podcast", "sql.db").Run()
		chErr(err)
		println("db inited")
	}

	db, err := sql.Open("sqlite3", dbName)
	dbErr(err)

	// read from stdin, only for test
	print("+1s: ")
	inp, err := bufio.NewReader(os.Stdin).ReadString('\n')
	chErr(err)

	// parse input
	urlIdx := strings.Index(inp, "http")
	newRec := []string{inp[:urlIdx-1], inp[urlIdx:]}

	// insert a new record
	query, err := db.Prepare("INSERT INTO podcast (name, url, created) VALUES(?,?,?)")
	chErr(err)

	resp, err := query.Exec(newRec[0], newRec[1], time.Now().Unix())
	chErr(err)

	newid, _ := resp.LastInsertId()
	print(newid)
	print(" is inserted.\n")

	// find my result

	// read from stdin, only for test
	print("search by: ")
	inp, err = bufio.NewReader(os.Stdin).ReadString('\n')
	chErr(err)

	// query search
	que := fmt.Sprint("SELECT name, url FROM podcast WHERE name LIKE '%", inp[:len(inp)-1], "%'")
	fmt.Println(que)
	rows, err := db.Query(que)
	chErr(err)

	for rows.Next() {
		var name string
		var url string

		err = rows.Scan(&name, &url)
		fmt.Println(name, url)
	}

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
