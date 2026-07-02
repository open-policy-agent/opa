/*
 * Base64 encoding/decoding (RFC1341)
 * Copyright (c) 2005-2019, Jouni Malinen <j@w1.fi>
 *
 *
 * This software may be distributed, used, and modified under the terms of
 * BSD license:
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 *
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * 3. Neither the name(s) of the above-listed copyright holder(s) nor the
 *    names of its contributors may be used to endorse or promote products
 *    derived from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <limits.h>

#include "json.h"
#include "malloc.h"
#include "value.h"

static const unsigned char base64_table[65] =
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static const unsigned char base64_url_table[65] =
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

static unsigned char * base64_gen_encode(const unsigned char *src, size_t len,
                                         size_t *out_len,
                                         const unsigned char *table,
                                         int add_pad)
{
	unsigned char *out, *pos;
	const unsigned char *end, *in;
	size_t olen;

	if (len >= SIZE_MAX / 4)
		return NULL;
	olen = len * 4 / 3 + 4; /* 3-byte blocks to 4-byte */
	olen++; /* nul termination */
	if (olen < len)
		return NULL; /* integer overflow */
	out = malloc(olen);
	if (out == NULL)
		return NULL;

	end = src + len;
	in = src;
	pos = out;
	while (end - in >= 3) {
		*pos++ = table[(in[0] >> 2) & 0x3f];
		*pos++ = table[(((in[0] & 0x03) << 4) | (in[1] >> 4)) & 0x3f];
		*pos++ = table[(((in[1] & 0x0f) << 2) | (in[2] >> 6)) & 0x3f];
		*pos++ = table[in[2] & 0x3f];
		in += 3;
	}

	if (end - in) {
		*pos++ = table[(in[0] >> 2) & 0x3f];
		if (end - in == 1) {
			*pos++ = table[((in[0] & 0x03) << 4) & 0x3f];
			if (add_pad)
				*pos++ = '=';
		} else {
			*pos++ = table[(((in[0] & 0x03) << 4) |
					(in[1] >> 4)) & 0x3f];
			*pos++ = table[((in[1] & 0x0f) << 2) & 0x3f];
		}
		if (add_pad)
			*pos++ = '=';
	}

	*pos = '\0';
	if (out_len)
		*out_len = pos - out;
	return out;
}


static unsigned char * base64_gen_decode(const unsigned char *src, size_t len,
                                         size_t *out_len,
                                         const unsigned char *table)
{
	unsigned char dtable[256], *out, *pos, block[4], tmp;
	size_t i, count, olen;
	int pad = 0;
	size_t extra_pad;

	memset(dtable, 0x80, 256);
	for (i = 0; i < sizeof(base64_table) - 1; i++)
		dtable[table[i]] = (unsigned char) i;
	dtable['='] = 0;

	count = 0;
	for (i = 0; i < len; i++) {
		if (dtable[src[i]] != 0x80)
			count++;
	}

	if (count == 0)
		return NULL;
	extra_pad = (4 - count % 4) % 4;

	olen = (count + extra_pad) / 4 * 3;
	pos = out = malloc(olen);
	if (out == NULL)
		return NULL;

	count = 0;
	for (i = 0; i < len + extra_pad; i++) {
		unsigned char val;

		if (i >= len)
			val = '=';
		else
			val = src[i];
		tmp = dtable[val];
		if (tmp == 0x80)
			continue;

		if (val == '=')
			pad++;
		block[count] = tmp;
		count++;
		if (count == 4) {
			*pos++ = (block[0] << 2) | (block[1] >> 4);
			*pos++ = (block[1] << 4) | (block[2] >> 2);
			*pos++ = (block[2] << 6) | block[3];
			count = 0;
			if (pad) {
				if (pad == 1)
					pos--;
				else if (pad == 2)
					pos -= 2;
				else {
					/* Invalid padding */
					free(out);
					return NULL;
				}
				break;
			}
		}
	}

	*out_len = pos - out;
	return out;
}


/**
 * base64_encode - Base64 encode
 * @src: Data to be encoded
 * @len: Length of the data to be encoded
 * @out_len: Pointer to output length variable, or %NULL if not used
 * Returns: Allocated buffer of out_len bytes of encoded data,
 * or %NULL on failure
 *
 * Caller is responsible for freeing the returned buffer. Returned buffer is
 * nul terminated to make it easier to use as a C string. The nul terminator is
 * not included in out_len.
 */
static unsigned char * base64_encode(const unsigned char *src, size_t len,
                                     size_t *out_len)
{
	return base64_gen_encode(src, len, out_len, base64_table, 1);
}


static unsigned char * base64_url_encode(const unsigned char *src, size_t len,
                                         size_t *out_len, int add_pad)
{
	return base64_gen_encode(src, len, out_len, base64_url_table, add_pad);
}


/**
 * base64_decode - Base64 decode
 * @src: Data to be decoded
 * @len: Length of the data to be decoded
 * @out_len: Pointer to output length variable
 * Returns: Allocated buffer of out_len bytes of decoded data,
 * or %NULL on failure
 *
 * Caller is responsible for freeing the returned buffer.
 */
static unsigned char * base64_decode(const unsigned char *src, size_t len,
                                     size_t *out_len)
{
	return base64_gen_decode(src, len, out_len, base64_table);
}


static unsigned char * base64_url_decode(const unsigned char *src, size_t len,
                                         size_t *out_len)
{
	return base64_gen_decode(src, len, out_len, base64_url_table);
}

OPA_BUILTIN
opa_value *opa_base64_is_valid(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return opa_boolean(false);
    }

    opa_string_t *s = opa_cast_string(a);
    size_t len;
    unsigned char *dec = base64_decode((const unsigned char*)s->v, s->len, &len);
    if (dec == NULL)
    {
        return opa_boolean(false);
    }

    free(dec);
    return opa_boolean(true);
}

OPA_BUILTIN
opa_value *opa_base64_decode(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    size_t len;
    char *dec = (char *)base64_decode((const unsigned char*)s->v, s->len, &len);
    return dec == NULL ? NULL : opa_string_allocated(dec, len);
}

OPA_BUILTIN
opa_value *opa_base64_encode(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    size_t len;
    char *enc = (char *)base64_encode((const unsigned char*)s->v, s->len, &len);
    return enc == NULL ? NULL : opa_string_allocated(enc, len);
}

OPA_BUILTIN
opa_value *opa_base64_url_decode(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    size_t len;
    char *dec = (char *)base64_url_decode((const unsigned char*)s->v, s->len, &len);
    return dec == NULL ? NULL : opa_string_allocated(dec, len);
}

OPA_BUILTIN
opa_value *opa_base64_url_encode(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    size_t len;
    char *enc = (char *)base64_url_encode((const unsigned char*)s->v, s->len, &len, 1);
    return enc == NULL ? NULL : opa_string_allocated(enc, len);
}

OPA_BUILTIN
opa_value *opa_json_unmarshal(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return NULL;
    }

    opa_string_t *s = opa_cast_string(a);
    return opa_json_parse(s->v, s->len);
}

OPA_BUILTIN
opa_value *opa_json_marshal(opa_value *a)
{
    const char *v = opa_json_dump(a);
    if (v == NULL)
    {
        return NULL;
    }

    return opa_string_allocated(v, strlen(v));
}

OPA_BUILTIN
opa_value *opa_json_is_valid(opa_value *a)
{
    if (opa_value_type(a) != OPA_STRING)
    {
        return opa_boolean(false);
    }


    opa_string_t *s = opa_cast_string(a);
    opa_value *r = opa_json_parse(s->v, s->len);

	if (r == NULL)
	{
		return opa_boolean(false);
	}

	opa_free(r);
	return opa_boolean(true);
}