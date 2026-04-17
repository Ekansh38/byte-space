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
		if data != "" {
			if len(s.history) == 0 || s.history[len(s.history)-1] != data {
				s.history = append(s.history, data)
			}
		}

		s.posInHistory = -1
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
	case "\x1b[A": // up arrow, most recent in history
		// save current buffer to history:
		if len(s.history) == 0 { // on empty history, nothing
			return ""
		}

		added := false
		if len(s.history) > 0 {
			if s.posInHistory == -1 {
				s.history = append(s.history, s.buffer)
				added = true
			} else if s.posInHistory >= 0 && s.posInHistory < len(s.history) {
				s.history[s.posInHistory] = s.buffer
			}
		} // add or update history.

		// change current buffer to the len of history -2, first minus to convert to index, second to skip the newly appended item
		if s.posInHistory == -1 { // top of history, default.
			vale := len(s.history) - 1
			if added {
				vale--
			}
			s.posInHistory = vale
			if s.posInHistory < 0 {
				s.posInHistory = -1
				return ""
			}
			// moves pos in history back with correct error checking.
		} else {
			if s.posInHistory > 0 {
				s.posInHistory--
			}
			// regular move back
		}

		s.buffer = s.history[s.posInHistory]
		prompt := fmt.Sprintf("%s$ %s", s.proc.CWD, s.buffer)
		s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\r\033[K%s", prompt)))
		s.cursorPosition = len(s.buffer)
		return ""
		// update buffer and things.

	case "\x1b[B": // only works if posInHistory != -1
		if len(s.history) == 0 {
			return ""
		} // if history is empty, NOTHING TO DO!

		if s.posInHistory == -1 {
			return ""
		} // if at end of history, nothing to do!!

		if s.posInHistory >= 0 && s.posInHistory < len(s.history) { // save value to buffer for updates.
			s.history[s.posInHistory] = s.buffer
		}

		if s.posInHistory == len(s.history)-2 { // minus 1 because to turn it to an index
			// most recent element
			// back to empty prompt
			s.buffer = s.history[len(s.history)-1] // into index, this gets the most recent half typed thingy.
			s.posInHistory = -1

			// exit early + update stuff
			prompt := fmt.Sprintf("%s$ %s", s.proc.CWD, s.buffer)
			s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\r\033[K%s", prompt)))
			s.cursorPosition = len(s.buffer)
			return ""
		}

		if s.posInHistory < len(s.history)-1 {
			s.posInHistory++
		} // regular move forward

		// updates + exit
		s.buffer = s.history[s.posInHistory]
		prompt := fmt.Sprintf("%s$ %s", s.proc.CWD, s.buffer)
		s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\r\033[K%s", prompt)))
		s.cursorPosition = len(s.buffer)
		return ""

	case "\x1b\x7f", "\x1bw", "\x15", "\x02", "\x1b": // handle arrow keys later, up and down for history
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
