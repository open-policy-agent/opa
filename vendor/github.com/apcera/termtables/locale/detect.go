// Copyright 2013 Apcera Inc. All rights reserved.

// +build !windows

package locale

/*
#include <langinfo.h>
#include <locale.h>

void
init_go_locale(void)
{
	(void) setlocale(LC_ALL, "");
}

const char *
get_charmap(void)
{
	return nl_langinfo(CODESET);
}

*/
import "C"

func init() {
	C.init_go_locale()
}

// GetCharmap returns the character map (aka CODESET) of the current locale,
// according to the system libc implementation.  The value is returned as a
// string.  Common values which might be seen include "US-ASCII" and "UTF-8".
func GetCharmap() string {
	return C.GoString(C.get_charmap())
}
