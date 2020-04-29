#include "unicode.h"

#include <stdio.h>

// Tests whether the code point is an utf-16 surrogate (encoded
// representation of low or high bits).
int opa_unicode_surrogate(int codepoint)
{
    return 0xd800 <= codepoint && codepoint < 0xe000 ? TRUE : FALSE;
}

// Reads the unicode UTF-16 code unit \uXXXX escaping.
int opa_unicode_decode_unit(const char *in, int i, int len)
{
    if (i+6 > len)
    {
        return -1;
    }

    if (in[i] != '\\' || in[i+1] != 'u')
    {
        return -1;
    }

    int codepoint = 0;

    for (int j = i+2; j < (i+6); j++)
    {
        char next = in[j];

        if ( '0' <= next && next <= '9') {
			next = next - '0';
        } else if ('a' <= next && next <= 'f') {
			next = next - 'a' + 10;
		} else if ('A' <= next && next <= 'F') {
			next = next - 'A' + 10;
        } else {
			return -1;
		}

		codepoint = codepoint * 16 + (int)next;
    }

    return codepoint;
}

// Translates an utf-16 surrogate pair to a code point.
int opa_unicode_decode_surrogate(int codepoint1, int codepoint2)
{
    if (!opa_unicode_surrogate(codepoint1) || !opa_unicode_surrogate(codepoint2))
    {
        return 0xfffd; // replacement char
    }

    return (codepoint1 - 0xd800) << 10 | (codepoint2 - 0xdc00) + 0x10000;
}

// Decodes UTF-8 character to a code point.
int opa_unicode_decode_utf8(const char *in, int i, int len, int *olen)
{
    if (i >= len)
    {
        return -1;
    }

    // For details, see https://en.wikipedia.org/wiki/UTF-8 and
    // https://lemire.me/blog/2018/05/09/how-quickly-can-you-check-that-a-string-is-valid-unicode-utf-8/

    unsigned char c0 = in[i];
    if ((c0 & 0b10000000) == 0)
    {
        // 1 byte UTF-8 character.
        return (int)c0;
    }

    if ((c0 & 0b11100000) == 0b11000000)
    {
        // 2 byte UTF-8 character.
        if ((i+1) >= len)
        {
            return -1;
        }

        // 0xc0 and 0xc1 are illegal UTF-8 first bytes, considered
        // overlong encodings.
        if (c0 == 0xc0 || c0 == 0xc1)
        {
            return -1;
        }

        unsigned char c1 = in[i+1];
        if (!(c1 >= 0x80 && c1 <= 0xbf))
        {
            return -1;
        }

        *olen = 2;
        return (int)(c0 & 0b00011111) << 6 | (int)(c1 & 0b00111111);
    }

    if ((c0 & 0b11110000) == 0b11100000)
    {
        // 3 byte UTF-8 character.
        if ((i+2) >= len)
        {
            return -1;
        }

        unsigned char c1 = in[i+1];
        unsigned char c2 = in[i+2];

        if (!((c0 == 0xe0 &&               c1 >= 0xa0 && c1 <= 0xbf && c2 >= 0x80 && c2 <= 0xbf) ||
              (c0 >= 0xe1 && c0 <= 0xec && c1 >= 0x80 && c1 <= 0xbf && c2 >= 0x80 && c2 <= 0xbf) ||
              (c0 == 0xed &&               c1 >= 0x80 && c1 <= 0x9f && c2 >= 0x80 && c2 <= 0xbf) ||
              (c0 >= 0xee && c0 <= 0xef && c1 >= 0x80 && c1 <= 0xbf && c2 >= 0x80 && c2 <= 0xbf)))
        {
            return -1;
        }

        *olen = 3;
        return (int)(c0 & 0b00001111) << 12 | (int)(c1 & 0b00111111) << 6 | (int)(c2 & 0b00111111);
    }

    if ((c0 & 0b11111000) == 0b11110000)
    {
        // 4 byte UTF-8 character.
        if ((i+3) >= len)
        {
            return -1;
        }

        unsigned char c1 = in[i+1];
        unsigned char c2 = in[i+2];
        unsigned char c3 = in[i+3];

        if (!((c0 == 0xf0 &&               c1 >= 0x90 && c1 <= 0xbf && c2 >= 0x80 && c2 <= 0xbf && c3 >= 0x80 && c3 <= 0xbf) ||
              (c0 >= 0xf1 && c0 <= 0xf3 && c1 >= 0x80 && c1 <= 0xbf && c2 >= 0x80 && c2 <= 0xbf && c3 >= 0x80 && c3 <= 0xbf) ||
              (c0 == 0xf4 &&               c1 >= 0x80 && c1 <= 0x8f && c2 >= 0x80 && c2 <= 0xbf && c3 >= 0x80 && c3 <= 0xbf)))
        {
            return -1;
        }

        *olen = 4;
        return (int)(c0 & 0b00000111) << 18 | (int)(c1 & 0b00111111) << 12 | (int)(c2 & 0b00111111) << 6 | (int)(c3 & 0b00111111);
    }

    return -1;
}

// Writes the code point as UTF-8.
int opa_unicode_encode_utf8(int codepoint, char *out)
{
    size_t i = (size_t)codepoint;

    if (i <= ((1<<7) - 1))
    {
        out[0] = i;
        return 1;
    }

    if (i <= ((1<<11) - 1))
    {
        out[0] = 0b11000000 | (i >> 6);
        out[1] = 0b10000000 | (i & 0b00111111);
        return 2;
    }

    if (i <= ((1<<16) - 1))
    {
        out[0] = 0b11100000 | (i >> 12);
        out[1] = 0b10000000 | ((i >> 6) & 0b00111111);
        out[2] = 0b10000000 | (i & 0b00111111);
        return 3;
    }

    out[0] = 0b11110000 | (i >> 18);
    out[1] = 0b10000000 | ((i >> 12) & 0b00111111);
    out[2] = 0b10000000 | ((i >> 6) & 0b00111111);
    out[3] = 0b10000000 | (i & 0b00111111);
    return 4;
}

