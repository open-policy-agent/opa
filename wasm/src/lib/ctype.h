#ifndef OPA_CTYPE_H
#define OPA_CTYPE_H

#ifdef __cplusplus
extern "C" {
#endif

#define	isascii(c)	(((c) & ~0x7f) == 0)

int isalpha(int c);
int isdigit(int c);
int islower(int c);
int isspace(int c);
int isupper(int c);
int tolower(int c);

#ifdef __cplusplus
}
#endif

#endif
