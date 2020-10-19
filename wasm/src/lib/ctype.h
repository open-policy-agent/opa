#ifndef OPA_CTYPE_H
#define OPA_CTYPE_H

#include <locale.h>

#ifdef __cplusplus
extern "C" {
#endif

#define	isascii(c)	(((c) & ~0x7f) == 0)

int isalpha(int c);
int islower(int c);
int isspace(int c);
int isupper(int c);
int tolower(int c);

// not implemented:

int isdigit(int c);
int toupper(int c);
int isalnum(int c);
int isblank(int c);
int iscntrl(int c);
int isgraph(int c);
int isprint(int c);
int ispunct(int c);
int isxdigit(int c);

#ifdef __cplusplus
}
#endif

#endif
