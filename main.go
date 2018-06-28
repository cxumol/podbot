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
	"encoding/xml"
	"strings"
	"time"

	"net/http"

	"unicode/utf8"

	"gopkg.in/telegram-bot-api.v4"
)

// to phrase xml

type Pcasts struct {
	Feeds []feed `xml:"body>outline>outline"`
}

type feed struct {
	Type string `xml:"type,attr"`
	Text string `xml:"text,attr"`
	Url  string `xml:"xmlUrl,attr"`
}

// lmx

// handle configue file
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

	bot.Debug = false

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
			case "delete":
				{
					if int64InSlice(int64(update.Message.From.ID), config.AdminID) {
						_, affectedrows := delItem(update.Message.Text, db)
						msg.Text = fmt.Sprintf("删除了%v条记录", affectedrows)
					}
				}
			default:
				msg.Text = "use /help to ask someone help you"
			}
			msg.ParseMode = tgbotapi.ModeMarkdown
			bot.Send(msg)
		} else if update.Message.Text != `` {

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			msg.ReplyToMessageID = update.Message.MessageID
			msg.Text = searchItem(update.Message.Text, db)
			if msg.Text == "" {
				msg.Text = "缺. 请试试换词重搜. \n欢迎补缺"
			}
			msg.ParseMode = tgbotapi.ModeMarkdown
			bot.Send(msg)
		} else if doc := update.Message.Document; doc != nil {

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "开始解析文件, 请耐心等待")
			msg.ReplyToMessageID = update.Message.MessageID

			log.Printf("found a file: %v\n", doc.FileName)
			pendingMsg, _ := bot.Send(msg)
			var updateMsgText string

			if strings.HasSuffix(doc.FileName, "xml") {
				fileURL, err := bot.GetFileDirectURL(doc.FileID)
				chErr(err)
				updateMsgText = opmlRead(fileURL, db)
			} else {
				updateMsgText = "只接受xml文件"
			}

			editedmsg := tgbotapi.NewEditMessageText(pendingMsg.Chat.ID, pendingMsg.MessageID, updateMsgText)
			bot.Send(editedmsg)
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
	if searchUrl(inp[urlIdx:], db) {
		return "", -1
	}
	// insert a new record
	query, err := db.Prepare("INSERT INTO podcast (name, url, created) VALUES(?,?,?)")
	chErr(err)

	// tx, err := db.Begin()
	// if err != nil {
	// 	fmt.Println(err)
	// }

	resp, err := query.Exec(newRec[0], newRec[1], time.Now().Unix())
	chErr(err)
	// resp, err := tx.Stmt(query).Exec(newRec[0], newRec[1], time.Now().Unix())
	// if err != nil {
	// 	fmt.Println("doing rollback")
	// 	tx.Rollback()
	// } else {
	// 	tx.Commit()
	// }

	newid, _ := resp.LastInsertId()

	return newRec[0], newid
}

func delItem(inp string, db *sql.DB) (string, int64) {
	// parse input
	begin_idx := strings.Index(inp, " ")
	que := fmt.Sprint("DELETE FROM podcast where name like '%", inp[begin_idx+1:], "%'")

	log.Println("deleting: " + inp[begin_idx+1:])

	query, err := db.Prepare(que)
	chErr(err)

	resp, err := query.Exec()
	chErr(err)

	affect, _ := resp.RowsAffected()

	return "deleted", affect
}

func searchUrl(inp string, db *sql.DB) bool {
	que := fmt.Sprint("SELECT name, url FROM podcast WHERE url = '", inp, "'")
	fmt.Println(que)
	rows, err := db.Query(que)
	chErr(err)

	defer rows.Close()
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
	defer rows.Close()
	for rows.Next() {
		var name string
		var url string

		err = rows.Scan(&name, &url)
		fmt.Println(name, url)
		result = result + name + " `" + url + "`\n"
	}
	return result
}

func opmlRead(opmlURL string, db *sql.DB) string {
	// newtmp()
	resp, err := http.Get(opmlURL)
	chErr(err)
	bxml, err := ioutil.ReadAll(resp.Body)
	chErr(err)

	urfeeds := Pcasts{}
	err = xml.Unmarshal(bxml, &urfeeds)
	if err != nil {
		fmt.Printf("error: %v", err)
		//return
	}

	// 读取完毕
	log.Println(urfeeds)
	if len(urfeeds.Feeds) == 0 {
		return "只支持 Pocket Casts 导出的文件"
	}
	// 开始导入
	for _, feed := range urfeeds.Feeds {
		if feed.Type == "rss" {

			inp := fmt.Sprintf("_add %v %v", feed.Text, feed.Url)
			chErr(err)

			itemName, reternID := addItem(inp, db)
			if reternID > 0 {
				log.Printf("登记 %s 于 %v", itemName, reternID)
			}

		}
	}
	return "导入成功"

}

func dbErr(err error) {
	if err != nil {
		println("You need initialize sql.db at first.")
		log.Fatal(err)
	}
}

func chErr(err error) {
	if err != nil {
		log.Println(err)
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

// func PathExists(path string) (bool, error) {
// 	_, err := os.Stat(path)
// 	if err == nil {
// 		return true, nil
// 	}
// 	if os.IsNotExist(err) {
// 		return false, nil
// 	}
// 	return false, err
// }

// func newtmp() {
// 	_dir := "./tmp"
// 	exist, err := PathExists(_dir)
// 	if err != nil {
// 		log.Printf("get dir error![%v]\n", err)
// 		return
// 	}

// 	if !exist {
// 		err := os.Mkdir(_dir, os.ModePerm)
// 		if err != nil {
// 			log.Printf("mkdir failed![%v]\n", err)
// 		} else {
// 			log.Printf("mkdir success!\n")
// 		}
// 	}
// }
