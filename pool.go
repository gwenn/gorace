package main

import (
	"github.com/gwenn/gosqlite"
)

const POOL_SIZE = 2

var connPool = make(chan *sqlite.Conn, POOL_SIZE)
var releaseChan = make(chan *sqlite.Conn)

func init() {
	go pool()
}

func pool() {
	for {
		db := <-releaseChan
		select {
		case connPool <- db:
			//trace(">Connection released in the pool")
		default:
			closeConn(db)
			//trace("Pool is full, connection closed")
		}
	}
}

func getConn() (db *sqlite.Conn, err error) {
	select {
	case db = <-connPool:
		//trace("Connection peek from pool")
	default:
		db, err = sqlite.Open(DB_PATH)
		if err != nil {
			fatal("Error while opening connection to %q: %s\n", DB_PATH, err)
			return
		}
		_, err = db.EnableFKey(true)
		// err = db.OneValue("PRAGMA journal_mode = wal", nil)
		//trace("Connection created")
	}
	return db, err
}
func releaseConn(db *sqlite.Conn) {
	select {
	case releaseChan <- db:
		//trace("<Connection released in the pool")
	default:
		closeConn(db)
		//trace("Connection closed")
	}
}

func closePool() {
	for {
		select {
		case db := <-connPool:
			closeConn(db)
		default:
			return
		}
	}
}
func closeConn(db *sqlite.Conn) {
	err := db.Close()
	if err != nil {
		warn("Error while closing connection: %s\n", err)
	}
}
