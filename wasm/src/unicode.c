#include "unicode.h"

#include <stdio.h>
#include <stdint.h>
#include "std.h"

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
        *olen = 1;
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

// Returns the index to the last UTF-8 code point.
int opa_unicode_last_utf8(const char *in, int start, int end)
{
    if (start >= end || end == 0)
    {
        return -1;
    }

    int i = end - 1;
    unsigned char c = in[i];
    if (c < 0x80)
    {
        return i;
    }

    for (i = i - 1; i >= start && i >= end - 4; i--) // UTF-8 character is at most 4 bytes.
    {
        c = in[i];
        if ((c & 0xc0) != 0x80) // Bytes after the first have always the two bits set to 10.
        {
            break;
        }
    }

    if (i < start) {
        i = start;
    }

    return i;
}

// The following tables mirror the implementation of golang unicode
// white space detection and case conversion routines.

typedef struct {
    uint16_t lo;
    uint16_t hi;
    uint16_t stride;
} range16_t;

// unicode white spaces expressed as ranges, per
// https://www.unicode.org/Public/UCD/latest/ucd/PropList.txt.
const static range16_t white_spaces[] = {
    {0x0009, 0x000d, 1},
    {0x0020, 0x0085, 101},
    {0x00a0, 0x1680, 5600},
    {0x2000, 0x200a, 1},
    {0x2028, 0x2029, 1},
    {0x202f, 0x205f, 48},
    {0x3000, 0x3000, 1},
};

// is16 reports whether codepoint is in the sorted slice of 16-bit ranges.
int is16(const range16_t *ranges, int n, uint16_t cp)
{
    for (int i = 0; i < n; i++)
    {
        if (cp < ranges[i].lo)
        {
            return FALSE;
        }

        if (cp <= ranges[i].hi)
        {
            return ranges[i].stride == 1 || (cp-ranges[i].lo)%ranges[i].stride == 0 ? TRUE : FALSE;
        }
    }

    return FALSE;
}

// is returns true if the codepoint is in the range table.
static int is(const range16_t *r16, int l16, int cp)
{
    if (l16 > 0 && cp <= r16[l16-1].hi)
    {
        return is16(r16, l16, (uint16_t)cp);
    }

    // TODO: Check for 32-bit ranges here, if such tables needed.
    // TODO: For any future larger tables, implement binary search.

    return FALSE;
}

// Returns true if the codepoint is a whitespace.
int opa_unicode_is_space(int cp)
{
    if (cp <= 0xff) { // Latin1
        return cp == '\t' || cp == '\n' || cp == '\v' || cp ==  '\f' || cp ==  '\r' || cp == ' ' || cp == 0x85 || cp == 0xa0;
    }

    return is(white_spaces, sizeof(white_spaces) / sizeof(range16_t), cp);
}

typedef struct {
    uint32_t lo;
    uint32_t hi;
    int d[3]; // delta
} case_range_t;

// Indices to d(elta) above.
#define UPPER_CASE 0
#define LOWER_CASE 1
#define TITLE_CASE 2

// These are the case conversion ranges from golang 1.14 (unicode
// 12.0.0).

static const int upper_lower = 0x10ffff + 1;
static const case_range_t case_ranges[] = {
    {0x0041, 0x005A, {0, 32, 0}},
    {0x0061, 0x007A, {-32, 0, -32}},
    {0x00B5, 0x00B5, {743, 0, 743}},
    {0x00C0, 0x00D6, {0, 32, 0}},
    {0x00D8, 0x00DE, {0, 32, 0}},
    {0x00E0, 0x00F6, {-32, 0, -32}},
    {0x00F8, 0x00FE, {-32, 0, -32}},
    {0x00FF, 0x00FF, {121, 0, 121}},
    {0x0100, 0x012F, {upper_lower, upper_lower, upper_lower}},
    {0x0130, 0x0130, {0, -199, 0}},
    {0x0131, 0x0131, {-232, 0, -232}},
    {0x0132, 0x0137, {upper_lower, upper_lower, upper_lower}},
    {0x0139, 0x0148, {upper_lower, upper_lower, upper_lower}},
    {0x014A, 0x0177, {upper_lower, upper_lower, upper_lower}},
    {0x0178, 0x0178, {0, -121, 0}},
    {0x0179, 0x017E, {upper_lower, upper_lower, upper_lower}},
    {0x017F, 0x017F, {-300, 0, -300}},
    {0x0180, 0x0180, {195, 0, 195}},
    {0x0181, 0x0181, {0, 210, 0}},
    {0x0182, 0x0185, {upper_lower, upper_lower, upper_lower}},
    {0x0186, 0x0186, {0, 206, 0}},
    {0x0187, 0x0188, {upper_lower, upper_lower, upper_lower}},
    {0x0189, 0x018A, {0, 205, 0}},
    {0x018B, 0x018C, {upper_lower, upper_lower, upper_lower}},
    {0x018E, 0x018E, {0, 79, 0}},
    {0x018F, 0x018F, {0, 202, 0}},
    {0x0190, 0x0190, {0, 203, 0}},
    {0x0191, 0x0192, {upper_lower, upper_lower, upper_lower}},
    {0x0193, 0x0193, {0, 205, 0}},
    {0x0194, 0x0194, {0, 207, 0}},
    {0x0195, 0x0195, {97, 0, 97}},
    {0x0196, 0x0196, {0, 211, 0}},
    {0x0197, 0x0197, {0, 209, 0}},
    {0x0198, 0x0199, {upper_lower, upper_lower, upper_lower}},
    {0x019A, 0x019A, {163, 0, 163}},
    {0x019C, 0x019C, {0, 211, 0}},
    {0x019D, 0x019D, {0, 213, 0}},
    {0x019E, 0x019E, {130, 0, 130}},
    {0x019F, 0x019F, {0, 214, 0}},
    {0x01A0, 0x01A5, {upper_lower, upper_lower, upper_lower}},
    {0x01A6, 0x01A6, {0, 218, 0}},
    {0x01A7, 0x01A8, {upper_lower, upper_lower, upper_lower}},
    {0x01A9, 0x01A9, {0, 218, 0}},
    {0x01AC, 0x01AD, {upper_lower, upper_lower, upper_lower}},
    {0x01AE, 0x01AE, {0, 218, 0}},
    {0x01AF, 0x01B0, {upper_lower, upper_lower, upper_lower}},
    {0x01B1, 0x01B2, {0, 217, 0}},
    {0x01B3, 0x01B6, {upper_lower, upper_lower, upper_lower}},
    {0x01B7, 0x01B7, {0, 219, 0}},
    {0x01B8, 0x01B9, {upper_lower, upper_lower, upper_lower}},
    {0x01BC, 0x01BD, {upper_lower, upper_lower, upper_lower}},
    {0x01BF, 0x01BF, {56, 0, 56}},
    {0x01C4, 0x01C4, {0, 2, 1}},
    {0x01C5, 0x01C5, {-1, 1, 0}},
    {0x01C6, 0x01C6, {-2, 0, -1}},
    {0x01C7, 0x01C7, {0, 2, 1}},
    {0x01C8, 0x01C8, {-1, 1, 0}},
    {0x01C9, 0x01C9, {-2, 0, -1}},
    {0x01CA, 0x01CA, {0, 2, 1}},
    {0x01CB, 0x01CB, {-1, 1, 0}},
    {0x01CC, 0x01CC, {-2, 0, -1}},
    {0x01CD, 0x01DC, {upper_lower, upper_lower, upper_lower}},
    {0x01DD, 0x01DD, {-79, 0, -79}},
    {0x01DE, 0x01EF, {upper_lower, upper_lower, upper_lower}},
    {0x01F1, 0x01F1, {0, 2, 1}},
    {0x01F2, 0x01F2, {-1, 1, 0}},
    {0x01F3, 0x01F3, {-2, 0, -1}},
    {0x01F4, 0x01F5, {upper_lower, upper_lower, upper_lower}},
    {0x01F6, 0x01F6, {0, -97, 0}},
    {0x01F7, 0x01F7, {0, -56, 0}},
    {0x01F8, 0x021F, {upper_lower, upper_lower, upper_lower}},
    {0x0220, 0x0220, {0, -130, 0}},
    {0x0222, 0x0233, {upper_lower, upper_lower, upper_lower}},
    {0x023A, 0x023A, {0, 10795, 0}},
    {0x023B, 0x023C, {upper_lower, upper_lower, upper_lower}},
    {0x023D, 0x023D, {0, -163, 0}},
    {0x023E, 0x023E, {0, 10792, 0}},
    {0x023F, 0x0240, {10815, 0, 10815}},
    {0x0241, 0x0242, {upper_lower, upper_lower, upper_lower}},
    {0x0243, 0x0243, {0, -195, 0}},
    {0x0244, 0x0244, {0, 69, 0}},
    {0x0245, 0x0245, {0, 71, 0}},
    {0x0246, 0x024F, {upper_lower, upper_lower, upper_lower}},
    {0x0250, 0x0250, {10783, 0, 10783}},
    {0x0251, 0x0251, {10780, 0, 10780}},
    {0x0252, 0x0252, {10782, 0, 10782}},
    {0x0253, 0x0253, {-210, 0, -210}},
    {0x0254, 0x0254, {-206, 0, -206}},
    {0x0256, 0x0257, {-205, 0, -205}},
    {0x0259, 0x0259, {-202, 0, -202}},
    {0x025B, 0x025B, {-203, 0, -203}},
    {0x025C, 0x025C, {42319, 0, 42319}},
    {0x0260, 0x0260, {-205, 0, -205}},
    {0x0261, 0x0261, {42315, 0, 42315}},
    {0x0263, 0x0263, {-207, 0, -207}},
    {0x0265, 0x0265, {42280, 0, 42280}},
    {0x0266, 0x0266, {42308, 0, 42308}},
    {0x0268, 0x0268, {-209, 0, -209}},
    {0x0269, 0x0269, {-211, 0, -211}},
    {0x026A, 0x026A, {42308, 0, 42308}},
    {0x026B, 0x026B, {10743, 0, 10743}},
    {0x026C, 0x026C, {42305, 0, 42305}},
    {0x026F, 0x026F, {-211, 0, -211}},
    {0x0271, 0x0271, {10749, 0, 10749}},
    {0x0272, 0x0272, {-213, 0, -213}},
    {0x0275, 0x0275, {-214, 0, -214}},
    {0x027D, 0x027D, {10727, 0, 10727}},
    {0x0280, 0x0280, {-218, 0, -218}},
    {0x0282, 0x0282, {42307, 0, 42307}},
    {0x0283, 0x0283, {-218, 0, -218}},
    {0x0287, 0x0287, {42282, 0, 42282}},
    {0x0288, 0x0288, {-218, 0, -218}},
    {0x0289, 0x0289, {-69, 0, -69}},
    {0x028A, 0x028B, {-217, 0, -217}},
    {0x028C, 0x028C, {-71, 0, -71}},
    {0x0292, 0x0292, {-219, 0, -219}},
    {0x029D, 0x029D, {42261, 0, 42261}},
    {0x029E, 0x029E, {42258, 0, 42258}},
    {0x0345, 0x0345, {84, 0, 84}},
    {0x0370, 0x0373, {upper_lower, upper_lower, upper_lower}},
    {0x0376, 0x0377, {upper_lower, upper_lower, upper_lower}},
    {0x037B, 0x037D, {130, 0, 130}},
    {0x037F, 0x037F, {0, 116, 0}},
    {0x0386, 0x0386, {0, 38, 0}},
    {0x0388, 0x038A, {0, 37, 0}},
    {0x038C, 0x038C, {0, 64, 0}},
    {0x038E, 0x038F, {0, 63, 0}},
    {0x0391, 0x03A1, {0, 32, 0}},
    {0x03A3, 0x03AB, {0, 32, 0}},
    {0x03AC, 0x03AC, {-38, 0, -38}},
    {0x03AD, 0x03AF, {-37, 0, -37}},
    {0x03B1, 0x03C1, {-32, 0, -32}},
    {0x03C2, 0x03C2, {-31, 0, -31}},
    {0x03C3, 0x03CB, {-32, 0, -32}},
    {0x03CC, 0x03CC, {-64, 0, -64}},
    {0x03CD, 0x03CE, {-63, 0, -63}},
    {0x03CF, 0x03CF, {0, 8, 0}},
    {0x03D0, 0x03D0, {-62, 0, -62}},
    {0x03D1, 0x03D1, {-57, 0, -57}},
    {0x03D5, 0x03D5, {-47, 0, -47}},
    {0x03D6, 0x03D6, {-54, 0, -54}},
    {0x03D7, 0x03D7, {-8, 0, -8}},
    {0x03D8, 0x03EF, {upper_lower, upper_lower, upper_lower}},
    {0x03F0, 0x03F0, {-86, 0, -86}},
    {0x03F1, 0x03F1, {-80, 0, -80}},
    {0x03F2, 0x03F2, {7, 0, 7}},
    {0x03F3, 0x03F3, {-116, 0, -116}},
    {0x03F4, 0x03F4, {0, -60, 0}},
    {0x03F5, 0x03F5, {-96, 0, -96}},
    {0x03F7, 0x03F8, {upper_lower, upper_lower, upper_lower}},
    {0x03F9, 0x03F9, {0, -7, 0}},
    {0x03FA, 0x03FB, {upper_lower, upper_lower, upper_lower}},
    {0x03FD, 0x03FF, {0, -130, 0}},
    {0x0400, 0x040F, {0, 80, 0}},
    {0x0410, 0x042F, {0, 32, 0}},
    {0x0430, 0x044F, {-32, 0, -32}},
    {0x0450, 0x045F, {-80, 0, -80}},
    {0x0460, 0x0481, {upper_lower, upper_lower, upper_lower}},
    {0x048A, 0x04BF, {upper_lower, upper_lower, upper_lower}},
    {0x04C0, 0x04C0, {0, 15, 0}},
    {0x04C1, 0x04CE, {upper_lower, upper_lower, upper_lower}},
    {0x04CF, 0x04CF, {-15, 0, -15}},
    {0x04D0, 0x052F, {upper_lower, upper_lower, upper_lower}},
    {0x0531, 0x0556, {0, 48, 0}},
    {0x0561, 0x0586, {-48, 0, -48}},
    {0x10A0, 0x10C5, {0, 7264, 0}},
    {0x10C7, 0x10C7, {0, 7264, 0}},
    {0x10CD, 0x10CD, {0, 7264, 0}},
    {0x10D0, 0x10FA, {3008, 0, 0}},
    {0x10FD, 0x10FF, {3008, 0, 0}},
    {0x13A0, 0x13EF, {0, 38864, 0}},
    {0x13F0, 0x13F5, {0, 8, 0}},
    {0x13F8, 0x13FD, {-8, 0, -8}},
    {0x1C80, 0x1C80, {-6254, 0, -6254}},
    {0x1C81, 0x1C81, {-6253, 0, -6253}},
    {0x1C82, 0x1C82, {-6244, 0, -6244}},
    {0x1C83, 0x1C84, {-6242, 0, -6242}},
    {0x1C85, 0x1C85, {-6243, 0, -6243}},
    {0x1C86, 0x1C86, {-6236, 0, -6236}},
    {0x1C87, 0x1C87, {-6181, 0, -6181}},
    {0x1C88, 0x1C88, {35266, 0, 35266}},
    {0x1C90, 0x1CBA, {0, -3008, 0}},
    {0x1CBD, 0x1CBF, {0, -3008, 0}},
    {0x1D79, 0x1D79, {35332, 0, 35332}},
    {0x1D7D, 0x1D7D, {3814, 0, 3814}},
    {0x1D8E, 0x1D8E, {35384, 0, 35384}},
    {0x1E00, 0x1E95, {upper_lower, upper_lower, upper_lower}},
    {0x1E9B, 0x1E9B, {-59, 0, -59}},
    {0x1E9E, 0x1E9E, {0, -7615, 0}},
    {0x1EA0, 0x1EFF, {upper_lower, upper_lower, upper_lower}},
    {0x1F00, 0x1F07, {8, 0, 8}},
    {0x1F08, 0x1F0F, {0, -8, 0}},
    {0x1F10, 0x1F15, {8, 0, 8}},
    {0x1F18, 0x1F1D, {0, -8, 0}},
    {0x1F20, 0x1F27, {8, 0, 8}},
    {0x1F28, 0x1F2F, {0, -8, 0}},
    {0x1F30, 0x1F37, {8, 0, 8}},
    {0x1F38, 0x1F3F, {0, -8, 0}},
    {0x1F40, 0x1F45, {8, 0, 8}},
    {0x1F48, 0x1F4D, {0, -8, 0}},
    {0x1F51, 0x1F51, {8, 0, 8}},
    {0x1F53, 0x1F53, {8, 0, 8}},
    {0x1F55, 0x1F55, {8, 0, 8}},
    {0x1F57, 0x1F57, {8, 0, 8}},
    {0x1F59, 0x1F59, {0, -8, 0}},
    {0x1F5B, 0x1F5B, {0, -8, 0}},
    {0x1F5D, 0x1F5D, {0, -8, 0}},
    {0x1F5F, 0x1F5F, {0, -8, 0}},
    {0x1F60, 0x1F67, {8, 0, 8}},
    {0x1F68, 0x1F6F, {0, -8, 0}},
    {0x1F70, 0x1F71, {74, 0, 74}},
    {0x1F72, 0x1F75, {86, 0, 86}},
    {0x1F76, 0x1F77, {100, 0, 100}},
    {0x1F78, 0x1F79, {128, 0, 128}},
    {0x1F7A, 0x1F7B, {112, 0, 112}},
    {0x1F7C, 0x1F7D, {126, 0, 126}},
    {0x1F80, 0x1F87, {8, 0, 8}},
    {0x1F88, 0x1F8F, {0, -8, 0}},
    {0x1F90, 0x1F97, {8, 0, 8}},
    {0x1F98, 0x1F9F, {0, -8, 0}},
    {0x1FA0, 0x1FA7, {8, 0, 8}},
    {0x1FA8, 0x1FAF, {0, -8, 0}},
    {0x1FB0, 0x1FB1, {8, 0, 8}},
    {0x1FB3, 0x1FB3, {9, 0, 9}},
    {0x1FB8, 0x1FB9, {0, -8, 0}},
    {0x1FBA, 0x1FBB, {0, -74, 0}},
    {0x1FBC, 0x1FBC, {0, -9, 0}},
    {0x1FBE, 0x1FBE, {-7205, 0, -7205}},
    {0x1FC3, 0x1FC3, {9, 0, 9}},
    {0x1FC8, 0x1FCB, {0, -86, 0}},
    {0x1FCC, 0x1FCC, {0, -9, 0}},
    {0x1FD0, 0x1FD1, {8, 0, 8}},
    {0x1FD8, 0x1FD9, {0, -8, 0}},
    {0x1FDA, 0x1FDB, {0, -100, 0}},
    {0x1FE0, 0x1FE1, {8, 0, 8}},
    {0x1FE5, 0x1FE5, {7, 0, 7}},
    {0x1FE8, 0x1FE9, {0, -8, 0}},
    {0x1FEA, 0x1FEB, {0, -112, 0}},
    {0x1FEC, 0x1FEC, {0, -7, 0}},
    {0x1FF3, 0x1FF3, {9, 0, 9}},
    {0x1FF8, 0x1FF9, {0, -128, 0}},
    {0x1FFA, 0x1FFB, {0, -126, 0}},
    {0x1FFC, 0x1FFC, {0, -9, 0}},
    {0x2126, 0x2126, {0, -7517, 0}},
    {0x212A, 0x212A, {0, -8383, 0}},
    {0x212B, 0x212B, {0, -8262, 0}},
    {0x2132, 0x2132, {0, 28, 0}},
    {0x214E, 0x214E, {-28, 0, -28}},
    {0x2160, 0x216F, {0, 16, 0}},
    {0x2170, 0x217F, {-16, 0, -16}},
    {0x2183, 0x2184, {upper_lower, upper_lower, upper_lower}},
    {0x24B6, 0x24CF, {0, 26, 0}},
    {0x24D0, 0x24E9, {-26, 0, -26}},
    {0x2C00, 0x2C2E, {0, 48, 0}},
    {0x2C30, 0x2C5E, {-48, 0, -48}},
    {0x2C60, 0x2C61, {upper_lower, upper_lower, upper_lower}},
    {0x2C62, 0x2C62, {0, -10743, 0}},
    {0x2C63, 0x2C63, {0, -3814, 0}},
    {0x2C64, 0x2C64, {0, -10727, 0}},
    {0x2C65, 0x2C65, {-10795, 0, -10795}},
    {0x2C66, 0x2C66, {-10792, 0, -10792}},
    {0x2C67, 0x2C6C, {upper_lower, upper_lower, upper_lower}},
    {0x2C6D, 0x2C6D, {0, -10780, 0}},
    {0x2C6E, 0x2C6E, {0, -10749, 0}},
    {0x2C6F, 0x2C6F, {0, -10783, 0}},
    {0x2C70, 0x2C70, {0, -10782, 0}},
    {0x2C72, 0x2C73, {upper_lower, upper_lower, upper_lower}},
    {0x2C75, 0x2C76, {upper_lower, upper_lower, upper_lower}},
    {0x2C7E, 0x2C7F, {0, -10815, 0}},
    {0x2C80, 0x2CE3, {upper_lower, upper_lower, upper_lower}},
    {0x2CEB, 0x2CEE, {upper_lower, upper_lower, upper_lower}},
    {0x2CF2, 0x2CF3, {upper_lower, upper_lower, upper_lower}},
    {0x2D00, 0x2D25, {-7264, 0, -7264}},
    {0x2D27, 0x2D27, {-7264, 0, -7264}},
    {0x2D2D, 0x2D2D, {-7264, 0, -7264}},
    {0xA640, 0xA66D, {upper_lower, upper_lower, upper_lower}},
    {0xA680, 0xA69B, {upper_lower, upper_lower, upper_lower}},
    {0xA722, 0xA72F, {upper_lower, upper_lower, upper_lower}},
    {0xA732, 0xA76F, {upper_lower, upper_lower, upper_lower}},
    {0xA779, 0xA77C, {upper_lower, upper_lower, upper_lower}},
    {0xA77D, 0xA77D, {0, -35332, 0}},
    {0xA77E, 0xA787, {upper_lower, upper_lower, upper_lower}},
    {0xA78B, 0xA78C, {upper_lower, upper_lower, upper_lower}},
    {0xA78D, 0xA78D, {0, -42280, 0}},
    {0xA790, 0xA793, {upper_lower, upper_lower, upper_lower}},
    {0xA794, 0xA794, {48, 0, 48}},
    {0xA796, 0xA7A9, {upper_lower, upper_lower, upper_lower}},
    {0xA7AA, 0xA7AA, {0, -42308, 0}},
    {0xA7AB, 0xA7AB, {0, -42319, 0}},
    {0xA7AC, 0xA7AC, {0, -42315, 0}},
    {0xA7AD, 0xA7AD, {0, -42305, 0}},
    {0xA7AE, 0xA7AE, {0, -42308, 0}},
    {0xA7B0, 0xA7B0, {0, -42258, 0}},
    {0xA7B1, 0xA7B1, {0, -42282, 0}},
    {0xA7B2, 0xA7B2, {0, -42261, 0}},
    {0xA7B3, 0xA7B3, {0, 928, 0}},
    {0xA7B4, 0xA7BF, {upper_lower, upper_lower, upper_lower}},
    {0xA7C2, 0xA7C3, {upper_lower, upper_lower, upper_lower}},
    {0xA7C4, 0xA7C4, {0, -48, 0}},
    {0xA7C5, 0xA7C5, {0, -42307, 0}},
    {0xA7C6, 0xA7C6, {0, -35384, 0}},
    {0xAB53, 0xAB53, {-928, 0, -928}},
    {0xAB70, 0xABBF, {-38864, 0, -38864}},
    {0xFF21, 0xFF3A, {0, 32, 0}},
    {0xFF41, 0xFF5A, {-32, 0, -32}},
    {0x10400, 0x10427, {0, 40, 0}},
    {0x10428, 0x1044F, {-40, 0, -40}},
    {0x104B0, 0x104D3, {0, 40, 0}},
    {0x104D8, 0x104FB, {-40, 0, -40}},
    {0x10C80, 0x10CB2, {0, 64, 0}},
    {0x10CC0, 0x10CF2, {-64, 0, -64}},
    {0x118A0, 0x118BF, {0, 32, 0}},
    {0x118C0, 0x118DF, {-32, 0, -32}},
    {0x16E40, 0x16E5F, {0, 32, 0}},
    {0x16E60, 0x16E7F, {-32, 0, -32}},
    {0x1E900, 0x1E921, {0, 34, 0}},
    {0x1E922, 0x1E943, {-34, 0, -34}},
};

// convert a codepoint to lower, upper, or title case, by binary
// searching the conversion instruction.
static int to(int case_, uint32_t cp)
{
    const case_range_t *range = case_ranges;

    for (int lo = 0, hi = sizeof(case_ranges) / sizeof(case_range_t); lo < hi; )
    {
        int m = lo + (hi-lo) / 2;

        if (range[m].lo <= cp && cp <= range[m].hi)
        {
            int delta = range[m].d[case_];

            if (delta > 0x10FFFF)
            {
                // In an Upper-Lower sequence, which always starts with
                // an UpperCase letter, the real deltas always look like:
                //  {0, 1, 0}    UpperCase (Lower is next)
                //  {-1, 0, -1}  LowerCase (Upper, Title are previous)
                // The characters at even offsets from the beginning of the
                // sequence are upper case; the ones at odd offsets are lower.
                // The correct mapping can be done by clearing or setting the low
                // bit in the sequence offset.
                // The constants UpperCase and TitleCase are even while LowerCase
                // is odd so we take the low bit from _case.
                return range[m].lo + (((cp-range[m].lo) & ~1) | (case_ & 1));
            }

            return cp + delta;
        }

        if (cp < range[m].lo)
        {
            hi = m;
        } else {
            lo = m + 1;
        }
    }

    return cp;
}

int opa_unicode_to_lower(int codepoint)
{
    return to(LOWER_CASE, codepoint);
}

int opa_unicode_to_upper(int codepoint)
{
    return to(UPPER_CASE, codepoint);
}
