#ifndef OPA_WCTYPE_H
#define OPA_WCTYPE_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef uint32_t wctype_t;
typedef const int32_t *wctrans_t;

// not implemented:

int iswalnum(wint_t wc);
int iswalpha(wint_t wc);
int iswblank(wint_t wc);
int iswcntrl(wint_t wc);
int iswctype(wint_t wc, wctype_t desc);
int iswdigit(wint_t wc);
int iswgraph(wint_t wc);
int iswlower(wint_t wc);
int iswprint(wint_t wc);
int iswpunct(wint_t wc);
int iswspace(wint_t wc);
int iswupper(wint_t wc);
int iswxdigit(wint_t wc);
wint_t towlower(wint_t wc);
wint_t towupper(wint_t wc);
wint_t towctrans(wint_t wc, wctrans_t desc);
wctype_t wctype(const char* property);
wctrans_t wctrans(const char* property);

#ifdef __cplusplus
}
#endif

#endif
