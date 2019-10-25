#include "std.h"

size_t opa_strlen(const char *s)
{
    const char *ptr = s;

    while (1)
    {
        if (*ptr == '\0')
        {
            return ptr - s;
        }

        ptr += 1;
    }
}

int opa_strncmp(const char *a, const char *b, int num)
{
    unsigned char *a1 = (unsigned char *)a;
    unsigned char *b1 = (unsigned char *)b;

    while (num--)
    {
        if (*a1 < *b1)
        {
            return -1;
        }
        else if (*a1 > *b1)
        {
            return 1;
        }
        a1++;
        b1++;
    }

    return 0;
}

int opa_strcmp(const char *a, const char *b)
{
    size_t len_a = opa_strlen(a);
    size_t len_b = opa_strlen(b);
    size_t min = len_a;

    if (len_b < min)
    {
        min = len_b;
    }

    unsigned char *a1 = (unsigned char *)a;
    unsigned char *b1 = (unsigned char *)b;

    for (int i = 0; i < min; i++)
    {
        if (a1[i] < b1[i])
        {
            return -1;
        }
        else if (a[i] > b[i])
        {
            return 1;
        }
    }

    if (len_a < len_b)
    {
        return -1;
    }
    else if (len_a > len_b)
    {
        return 1;
    }
    return 0;
}

int opa_isdigit(char b)
{
    return b >= '0' && b <= '9';
}

int opa_isspace(char b)
{
    return b == ' ' || b == '\r' || b == '\n' || b == '\t';
}

int opa_ishex(char b)
{
    return opa_isdigit(b) || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f');
}

char *opa_reverse(char *str)
{
    size_t n = opa_strlen(str)-1;

    if (n <= 0)
    {
        return str;
    }

    int i = 0;

    while (i < n)
    {
        char tmp = str[i];
        str[i] = str[n];
        str[n] = tmp;

        i++;
        n--;
    }

    return str;
}

const char *digits = "0123456789abcdef";

char *opa_itoa(long long i, char *str, int base)
{
    char *buf = str;
    int is_negative = 0;

    if (i < 0)
    {
        is_negative = 1;
        i = -i;
    }

    do
    {
        int x = i % base;
        *buf++ = digits[x];
        i /= base;
    }
    while (i > 0);

    if (is_negative)
    {
        *buf++ = '-';
    }

    *buf++ = 0;

    return opa_reverse(str);
}