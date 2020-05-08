#include "string.h"

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

int opa_atoi64(const char *str, int len, long long *result)
{
    if (len <= 0)
    {
        return -1;
    }

    int i = 0;
    int sign = 1;

    if (str[i] == '-')
    {
        sign = -1;
        i++;
    }

    long long n = 0;

    for (; i < len; i++)
    {
        if (!opa_isdigit(str[i]))
        {
            return -2;
        }

        n = (n * 10) + (long long)(str[i] - '0');
    }

    *result = n * sign;

    return 0;
}

int opa_atof64(const char *str, int len, double *result)
{
    if (len <= 0)
    {
        return -1;
    }

    // Handle sign.
    double sign = 1.0;
    int i = 0;

    if (str[i] == '-')
    {
        sign = -1.0;
        i++;
    }

    // Handle integer component.
    double d = 0.0;

    for (; i < len && opa_isdigit(str[i]); i++)
    {
        d = (10.0 * d) + (double)(str[i] - '0');
    }

    d *= sign;

    if (i == len)
    {
        *result = d;
        return 0;
    }

    // Handle fraction component.
    if (str[i] == '.')
    {
        i++;

        double b = 0.1;
        double frac = 0;

        for (; i < len && opa_isdigit(str[i]); i++)
        {
            frac += b * (str[i] - '0');
            b /= 10.0;
        }

        d += (frac * sign);

        if (i == len)
        {
            *result = d;
            return 0;
        }

    }

    // Handle exponent component.
    if (str[i] == 'e' || str[i] == 'E')
    {
        i++;
        int exp_sign = 1;

        if (str[i] == '-')
        {
            exp_sign = -1;
            i++;
        }
        else if (str[i] == '+')
        {
            i++;
        }

        int e = 0;

        for (; i < len && opa_isdigit(str[i]); i++)
        {
            e = 10 * e + (int)(str[i] - '0');
        }

        if (i == len)
        {
            // Calculate pow(10, e).
            int x = 1;

            for (; e > 0; e--)
            {
                x *= 10;
            }

            *result = d * (double)(exp_sign * x);
            return 0;
        }
    }

    return -2;
}
