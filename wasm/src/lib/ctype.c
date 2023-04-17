#include <ctype.h>

/* POSIX/C local implementations. */

int isalpha(int c)
{
    return isupper(c) || islower(c);
}

int islower(int c)
{
    return c >= 'a' && c <= 'z' ? 1 : 0;
}

int isspace(int c)
{
    return c == ' ' || c == '\f' || c == '\n' || c == '\r' || c == '\t' || c == '\v';
}

int isupper(int c)
{
    return c >= 'A' && c <= 'Z' ? 1 : 0;
}

int tolower(int c)
{
    if (isupper(c))
    {
        return c + ('a' - 'A');
    }

    return c;
}
