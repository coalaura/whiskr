package main

func debug(format string, args ...any) {
	if !Debug {
		return
	}

	log.Debugf(format+"\n", args...)
}

func debugIf(cond bool, format string, args ...any) {
	if !cond {
		return
	}

	debug(format, args)
}
