package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	"io/ioutil"
	"os"
	"os/exec"

	"fmt"
	"log"

	"encoding/json"
	"strings"
	"time"

	"unicode/utf8"

	"gopkg.in/telegram-bot-api.v4"
)

type Config struct {
	BotApi  string  `json:"BotApi"`
	AdminID []int64 `json:"AdminID"`
	GroupID []int64 `json:"GroupID"`
}

func loadConfig(confFile string) Config {
	// confFile:="./config.json"

	raw, err := ioutil.ReadFile(confFile)
	chErr(err)

	var c Config
	json.Unmarshal(raw, &c)
	return c
}

func loadDB(dbName string) *sql.DB {
	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		err := exec.Command("cp", "sql.db.init4podcast", "sql.db").Run()
		chErr(err)
		println("db inited")
	}

	db, err := sql.Open("sqlite3", dbName)
	dbErr(err)
	return db
}

func loadtgbot(botApi string) (*tgbotapi.BotAPI, tgbotapi.UpdatesChannel) {
	bot, err := tgbotapi.NewBotAPI(botApi)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	return bot, updates
}

func main() {

	// load configure, database, and telegram bot
	config := loadConfig("./config.json")
	db := loadDB("./sql.db")

	// println(config.BotApi)
	// load bot
	bot, updates := loadtgbot(config.BotApi)
	botHandler(bot, updates, db, config)

}

func botHandler(bot *tgbotapi.BotAPI, updates tgbotapi.UpdatesChannel, db *sql.DB, config Config) {
	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.Chat.ID, update.Message.Text)

		if update.Message.IsCommand() {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			switch update.Message.Command() {
			case "add":
				{
					if int64InSlice(update.Message.Chat.ID, config.GroupID) {
						itemName, itemid := addItem(update.Message.Text, db)
						switch itemid {
						case -1:
							msg.Text = "劳驾先搜索, 免得重复添加"
						case -2:
							msg.Text = "链接呢???"
						case -3:
							msg.Text = "检查出无效字符, 无法添加"
						default:
							msg.Text = fmt.Sprintf("登记了 %s . ", itemName)
						}
					} else {
						msg.Text = "只能在指定场合新增"
					}

				}
			case "help":
				msg.Text = "Sorry I can't help you. \nYou have to help yourself."
			default:
				msg.Text = "use /help to ask someone help you"
			}
			bot.Send(msg)
		} else {

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			msg.ReplyToMessageID = update.Message.MessageID
			msg.Text = searchItem(update.Message.Text, db)
			if msg.Text == "" {
				msg.Text = "缺. 请试试换词重搜. \n欢迎补缺"
			}
			bot.Send(msg)
		}
	}
}

func addItem(inp string, db *sql.DB) (string, int64) {
	// parse input

	// check if input is valid utf8
	if utf8.ValidString(inp) == false {
		return "", -3
	}

	// check if contain url
	urlIdx := strings.Index(inp, "http")
	if urlIdx == -1 {
		return "", -2
	}

	first_idx := strings.Index(inp, " ")
	newRec := []string{inp[first_idx+1 : urlIdx-1], inp[urlIdx:]}

	// check duplicated record
	if searchUrl(inp, db) {
		return "", -1
	}
	// insert a new record
	query, err := db.Prepare("INSERT INTO podcast (name, url, created) VALUES(?,?,?)")
	chErr(err)

	resp, err := query.Exec(newRec[0], newRec[1], time.Now().Unix())
	chErr(err)

	newid, _ := resp.LastInsertId()

	return newRec[0], newid
}

func searchUrl(inp string, db *sql.DB) bool {
	que := fmt.Sprint("SELECT name, url FROM podcast WHERE url = '", inp, "'")
	fmt.Println(que)
	rows, err := db.Query(que)
	chErr(err)

	for rows.Next() {
		var name string
		var url string = ""

		err = rows.Scan(&name, &url)
		chErr(err)
		if url != "" {
			return true
		}
	}
	return false
}

func searchItem(inp string, db *sql.DB) string {
	que := fmt.Sprint("SELECT name, url FROM podcast WHERE name LIKE '%", inp, "%'")
	fmt.Println(que)
	rows, err := db.Query(que)
	chErr(err)

	result := ``
	for rows.Next() {
		var name string
		var url string

		err = rows.Scan(&name, &url)
		fmt.Println(name, url)
		result = result + name + " `" + url + "`\n"
	}
	return result
}

func dbErr(err error) {
	if err != nil {
		println("You need initialize sql.db at first.")
		log.Fatal(err)
	}
}

func chErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func int64InSlice(a int64, list []int64) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
