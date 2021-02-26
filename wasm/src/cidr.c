#include <stddef.h>
#include <string.h>

#include "std.h"
#include "str.h"
#include "value.h"

typedef struct {
    uint8_t len;
    unsigned char ip[16];
    unsigned char mask[16];
} ip_net;

typedef unsigned char u_char;
typedef unsigned int u_int;

static int inet_pton4(const char *src, const char *end, u_char *dst);
static int inet_pton6(const char *src, const char *end, u_char *dst);

static bool parse_ip(const char *src, int n, ip_net *dst)
{
    for (int i = 0; i < n; i++) {
        if (src[i] == '.')
        {
            dst->len = 4;
            memset(dst->mask, 0xff, 4);
            return inet_pton4(src, src + n, dst->ip);
        }
        else if (src[i] == ':')
        {
            dst->len = 16;
            memset(dst->mask, 0xff, 16);
            return inet_pton6(src, src + n, dst->ip);
        }
    }

    return false;
}

static bool parse_cidr(const char *src, size_t n, ip_net *dst)
{
    const char *slash = NULL;
    for (size_t i = 0; i < n; i++)
    {
        if (src[i] == '/')
        {
            slash = &src[i];
            break;
        }
    }

    if (slash == NULL)
    {
        return false;
    }

    const char *addr = src;
    const size_t len = slash - src;
    if (!parse_ip(addr, len, dst))
    {
        return false;
    }

    const char *mask = slash + 1;
    long long bits;
    if (opa_atoi64(mask, n - len - 1, &bits) == -1 || bits < 0 || bits > dst->len*8)
    {
        return false;
    }

    for (int i = 0; i < dst->len; i++)
    {
        if (bits >= 8) {
            dst->mask[i] = 0xff;
            bits -= 8;
            continue;
        }

        dst->mask[i] = ~((unsigned char)(0xff >> bits));
        bits = 0;
    }

    for (int i = 0; i < dst->len; i++)
    {
        dst->ip[i] &= dst->mask[i];
    }

    return true;
}

// returns true if a contains b.
static bool contains(ip_net *a, ip_net *b)
{
    if (a->len != b->len)
    {
        return false;
    }

    for (int i = 0; i < a->len; i++)
    {
        if (a->mask[i] & ~b->mask[i])
        {
            // If b mask is shorter, a can never contain b. For
            // example, 192.168/16 (a) doesn't contain 192.168/15 (b).
            // The above logical operation checks if the b mask is
            // shorter. For example, consider the masks a and b:
            //
            //        |   mask
            // -------+----------
            //   a    | 11111000
            //   b    | 11110000
            //   ~b   | 00001111
            // a & ~b | 00001000
            //
            return false;
        }

        // Since b mask may be longer than a, use the a mask to ignore
        // the b bits not relevant for comparison. Note, ips are
        // already masked at the construction time with their own
        // masks (see L84 above) so they don't have bits beyond their
        // mask length.
        if (a->ip[i] != (b->ip[i] & a->mask[i]))
        {
            return false;
        }
    }

    return true;
}

OPA_BUILTIN
opa_value *opa_cidr_contains(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    ip_net ip_a, ip_b;

    opa_string_t *s = opa_cast_string(a);
    if (!parse_cidr(s->v, s->len, &ip_a)) {
        return NULL;
    }

    s = opa_cast_string(b);
    if (!parse_ip(s->v, s->len, &ip_b) && !parse_cidr(s->v, s->len, &ip_b)) {
        return NULL;
    }

    return opa_boolean(contains(&ip_a, &ip_b));
}

OPA_BUILTIN
opa_value *opa_cidr_intersects(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_STRING || opa_value_type(b) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *as = opa_cast_string(a);
    opa_string_t *bs = opa_cast_string(b);
    ip_net ip_a, ip_b;
    if (!parse_cidr(as->v, as->len, &ip_a) || !parse_cidr(bs->v, bs->len, &ip_b)) {
        return NULL;
    }

    return opa_boolean(contains(&ip_a, &ip_b) || contains(&ip_b, &ip_a));
}

/*
 * Copyright (c) 2004 by Internet Systems Consortium, Inc. ("ISC")
 * Copyright (c) 1996,1999 by Internet Software Consortium.
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND ISC DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS.  IN NO EVENT SHALL ISC BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT
 * OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

/* int
 * inet_pton4(src, dst)
 *	like inet_aton() but without all the hexadecimal and shorthand.
 * return:
 *	1 if `src' is a valid dotted quad, else 0.
 * notice:
 *	does not touch `dst' unless it's returning 1.
 * author:
 *  Paul Vixie, 1996.
 */
static int
inet_pton4(const char *src, const char *stop, u_char *dst)
{
    static const char digits[] = "0123456789";
    int saw_digit, octets, ch;
#define NS_INADDRSZ     4
    u_char tmp[NS_INADDRSZ], *tp;

    saw_digit = 0;
    octets = 0;
    *(tp = tmp) = 0;
    while (src != stop && (ch = *src++) != '\0') {
        const char *pch;

        if ((pch = strchr(digits, ch)) != NULL) {
            u_int new = *tp * 10 + (pch - digits);

            if (saw_digit && *tp == 0)
                return (0);
            if (new > 255)
                return (0);
            *tp = new;
            if (!saw_digit) {
                if (++octets > 4)
                    return (0);
                saw_digit = 1;
            }
        } else if (ch == '.' && saw_digit) {
            if (octets == 4)
                return (0);
            *++tp = 0;
            saw_digit = 0;
        } else
            return (0);
    }
    if (octets < 4)
        return (0);
    memcpy(dst, tmp, NS_INADDRSZ);
    return (1);
}

/* int
 * inet_pton6(src, dst)
 *  convert presentation level address to network order binary form.
 * return:
 *  1 if `src' is a valid [RFC1884 2.2] address, else 0.
 * notice:
 *  (1) does not touch `dst' unless it's returning 1.
 *  (2) :: in a full address is silently ignored.
 * credit:
 *  inspired by Mark Andrews.
 * author:
 *  Paul Vixie, 1996.
 */
static int
inet_pton6(const char *src, const char *stop, u_char *dst)
{
    static const char xdigits_l[] = "0123456789abcdef",
        xdigits_u[] = "0123456789ABCDEF";
#define NS_IN6ADDRSZ    16
#define NS_INT16SZ  2
    u_char tmp[NS_IN6ADDRSZ], *tp, *endp, *colonp;
    const char *xdigits, *curtok, *end;
    int ch, seen_xdigits;
    u_int val;

    memset((tp = tmp), '\0', NS_IN6ADDRSZ);
    endp = tp + NS_IN6ADDRSZ;
    colonp = NULL;
    /* Leading :: requires some special handling. */
    if (*src == ':')
        if (*++src != ':')
            return (0);
    curtok = src;
    seen_xdigits = 0;
    val = 0;
    while (src != stop && (ch = *src++) != '\0') {
        const char *pch;

        if ((pch = strchr((xdigits = xdigits_l), ch)) == NULL)
            pch = strchr((xdigits = xdigits_u), ch);
        if (pch != NULL) {
            val <<= 4;
            val |= (pch - xdigits);
            if (++seen_xdigits > 4)
                return (0);
            continue;
        }
        if (ch == ':') {
            curtok = src;
            if (!seen_xdigits) {
                if (colonp)
                    return (0);
                colonp = tp;
                continue;
            } else if (*src == '\0') {
                return (0);
            }
            if (tp + NS_INT16SZ > endp)
                return (0);
            *tp++ = (u_char) (val >> 8) & 0xff;
            *tp++ = (u_char) val & 0xff;
            seen_xdigits = 0;
            val = 0;
            continue;
        }
        if (ch == '.' && ((tp + NS_INADDRSZ) <= endp) &&
            inet_pton4(curtok, stop, tp) > 0) {
            tp += NS_INADDRSZ;
            seen_xdigits = 0;
            break;  /*%< '\\0' was seen by inet_pton4(). */
        }
        return (0);
    }
    if (seen_xdigits) {
        if (tp + NS_INT16SZ > endp)
            return (0);
        *tp++ = (u_char) (val >> 8) & 0xff;
        *tp++ = (u_char) val & 0xff;
    }
    if (colonp != NULL) {
        /*
         * Since some memmove()'s erroneously fail to handle
         * overlapping regions, we'll do the shift by hand.
         */
        const int n = tp - colonp;
        int i;

        if (tp == endp)
            return (0);
        for (i = 1; i <= n; i++) {
            endp[- i] = colonp[n - i];
            colonp[n - i] = 0;
        }
        tp = endp;
    }
    if (tp != endp)
        return (0);
    memcpy(dst, tmp, NS_IN6ADDRSZ);
    return (1);
}
