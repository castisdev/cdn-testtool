package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Request describes request xml
type Request struct {
	externalID  string
	commandID   string
	reqType     string
	reqSubtype  string
	date        string
	contentName string
	from        string
	to          string
}

func main() {
	var addr, dbAddr, dbName, dbUser, dbPass string
	flag.StringVar(&addr, "addr", "0.0.0.0:8082", "listen address")
	flag.StringVar(&dbAddr, "db-addr", "", "mysql DB  address, need only for kt-simul, (ex)172.16.232.23:3306")
	flag.StringVar(&dbName, "db-name", "kt_test", "database name, need only for kt-simul")
	flag.StringVar(&dbUser, "db-user", "", "database user, need only for kt-simul")
	flag.StringVar(&dbPass, "db-pass", "", "database pass, need only for kt-simul")
	flag.Parse()

	if dbAddr != "" {
		if err := checkDB(dbAddr, dbName, dbUser, dbPass); err != nil {
			log.Fatalf("failed to connect DB, %v", err)
		}
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen, %v", err)
	}
	defer l.Close()
	log.Println("listening on " + addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("failed to accept, %v", err)
		}
		go handleRequest(conn, dbAddr, dbName, dbUser, dbPass)
	}
}

func parseRequest(text string) *Request {
	req := new(Request)
	words := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '<', '>', ' ', '/':
			return true
		}
		return false
	})

	curTime := time.Now().Local()
	req.date = curTime.Format("2006-01-02 15:04:05")

	for i, w := range words {
		switch {
		case strings.HasPrefix(w, "External_ID="):
			req.externalID = w[13 : len(w)-1]
		case strings.HasPrefix(w, "Command_ID="):
			req.commandID = w[12 : len(w)-1]
		case strings.HasPrefix(w, "Type="):
			req.reqType = w[6 : len(w)-1]
		case strings.HasPrefix(w, "Subtype="):
			req.reqSubtype = w[9 : len(w)-1]
		case strings.HasPrefix(w, "Name=\"ContentName\""):
			req.contentName = words[i+1][7 : len(words[i+1])-1]
		case strings.HasPrefix(w, "Name=\"From\""):
			req.from = words[i+1][7 : len(words[i+1])-1]
		case strings.HasPrefix(w, "Name=\"To\""):
			req.to = words[i+1][7 : len(words[i+1])-1]
		}
	}
	return req
}

func resonseXML(req Request) string {
	r := `<?xml version="1.0" encoding="euc-kr"?>
<!DOCTYPE Castanets SYSTEM "CastanetsGSM.DTD">
<Castanets>
	<Command External_ID="` + req.externalID + `" Command_ID="` + req.commandID +
		`" Type="` + req.reqType + `" Subtype="Response" Date="` + req.date + `">
		<Command_Data Name="ResultCode" Value="100"></Command_Data>
		<Command_Data Name="Description" Value="Completed"></Command_Data>
		<Command_Data Name="ContentName" Value="` + req.contentName + `"></Command_Data>
		<Command_Data Name="From" Value="` + req.from + `"></Command_Data>`
	if req.reqType == "UpdateCopyContent" {
		r += "\n" + `		<Command_Data Name="To" Value="` + req.to + `"></Command_Data>`
	}
	r += "\n" + `	</Command>` + "\n" + `</Castanets>`
	return r
}

func handleRequest(conn net.Conn, dbAddr, dbName, dbUser, dbPass string) {
	buf := make([]byte, 4096)
	_, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("failed to read msg, %v", err)
	}
	log.Println("Request received.")
	log.Println(string(buf))
	req := parseRequest(string(buf))
	res := resonseXML(*req)
	conn.Write([]byte(res))
	log.Printf("\nResponse sended. %v\n", req.contentName)
	log.Printf("%s\n\n", res)
	conn.Close()
	if dbAddr != "" {
		if err := updateDB(dbAddr, dbName, dbUser, dbPass, req); err != nil {
			log.Fatalf("failed to update DB, %v", err)
		}
	}
}

func checkDB(dbAddr, dbName, dbUser, dbPass string) error {
	url := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPass, dbAddr, dbName)
	db, err := sql.Open("mysql", url)
	if err != nil {
		return err
	}
	defer db.Close()
	return nil

}
func updateDB(dbAddr, dbName, dbUser, dbPass string, req *Request) error {
	if req.reqType == "UpdateCopyContent" {
		url := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPass, dbAddr, dbName)
		db, err := sql.Open("mysql", url)
		if err != nil {
			return err
		}
		defer db.Close()
		isHot := req.to == "Local"
		_, err = db.Exec("UPDATE service_content SET is_hot=? WHERE file=?;", isHot, req.contentName)
		if err == nil {
			log.Printf("DB Updated, %v\n", req.contentName)
		}
		return err
	}
	return nil
}
