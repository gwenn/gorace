package main

import (
	"log"
)

const TRACE = true

func fatal(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func warn(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func trace(msg string) {
	if TRACE {
		log.Println(msg)
	}
}
func tracef(format string, v ...interface{}) {
	if TRACE {
		log.Printf(format, v...)
	}
}
