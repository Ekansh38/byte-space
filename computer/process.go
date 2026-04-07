package computer

type Process struct {
	PID  int
	UID  string // who launched it (session user)
	EUID string // effective user (Owner if setuid or then UID)
	CWD  string // working dir snapshot at exec time
	PGID int

	Program Program
}
