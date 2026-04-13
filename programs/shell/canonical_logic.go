package shell

import (
	"fmt"
	"strings"
)

// btw now shell buffer stuff wont apear in TUI which is sad, but i dont really care because the TUI is vibe coded and...
// it doesnt really matter

func (s *Shell) canonicalLogic(receivedData string) string {
	if strings.HasPrefix(receivedData, "\x1b[1;5") { // incomplete key.
		return ""
	}

	if len(receivedData) == 1 && receivedData[0] == ';' {
		return ""
	}

	preCursor := s.cursorPosition
	ansiData := receivedData

	if receivedData == "\x7f" { // delete into backspace BANAANSI
		if s.cursorPosition > 0 {
			ansiData = "\b \b"
			s.cursorPosition--
		} else {
			ansiData = ""
		}
	} else if receivedData == "\t" {
		ansiData = "    " // expand tab to 4 spaces visually
	} else if receivedData == "\x1b[A" || receivedData == "\x1b[B" || receivedData == "\x15" {
		ansiData = ""
	} else if receivedData == "\x1b[C" {
		if s.cursorPosition == len(s.buffer) {
			ansiData = ""
		}
	} else if receivedData == "\x1b[D" {
		if s.cursorPosition == 0 {
			ansiData = ""
		}
	}

	s.Kernel.Write(s.proc, 1, []byte(ansiData))

	switch receivedData {

	case "\r": // enter
		data := s.buffer

		s.cursorPosition = 0
		s.buffer = ""
		return data

	case "\x7f": // delete
		runes := []rune(s.buffer)

		if preCursor > 0 && s.cursorPosition < len(runes) {
			runes = append(runes[:s.cursorPosition], runes[s.cursorPosition+1:]...)
			s.buffer = string(runes)

			right := string(runes[s.cursorPosition:])
			if len(right) > 0 {
				output := right + " "
				output += fmt.Sprintf("\x1b[%dD", len(right)+1)
				s.Kernel.Write(s.proc, 1, []byte(output))
			}
		}

	case "\x1b[C":
		if s.cursorPosition != len(s.buffer) {
			s.cursorPosition += 1
		}

	case "\x1b[D":
		if s.cursorPosition != 0 {
			s.cursorPosition -= 1
		}
	case "\x1b[A", "\x1b[B", "\x1b\x7f", "\x1bw", "\x15", "\x02", "\x1b": // handle arrow keys later, up and down for history
		return ""
	default:

		index := s.cursorPosition
		data := receivedData
		if data == "\t" {
			data = "    " // expand tab to 4 spaces in buffer
		}

		r := []rune(data)

		runes := []rune(s.buffer)
		runes = append(runes[:index], append(r, runes[index:]...)...)
		newStr := string(runes)
		s.buffer = newStr
		s.cursorPosition += len(r)
		right := string(runes[s.cursorPosition:])
		if len(right) > 0 {
			output := right + " "
			output += fmt.Sprintf("\x1b[%dD", len(right)+1)
			s.Kernel.Write(s.proc, 1, []byte(output))
		}

	}

	return ""
}
