#include "math.h"

#include <stdint.h>

/*
 * ====================================================
 * Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
 *
 * Developed at SunPro, a Sun Microsystems, Inc. business.
 * Permission to use, copy, modify, and distribute this
 * software is freely granted, provided that this notice
 * is preserved.
 * ====================================================
 */

typedef union
{
  double value;
  struct
  {
    uint32_t lsw;
    uint32_t msw;
  } parts;
  struct
  {
    uint64_t w;
  } xparts;
} ieee_double_shape_type;

/* Get two 32 bit ints from a double.  */

#define EXTRACT_WORDS(ix0,ix1,d)                \
    do {                                        \
        ieee_double_shape_type ew_u;            \
        ew_u.value = (d);                       \
        (ix0) = ew_u.parts.msw;                 \
        (ix1) = ew_u.parts.lsw;                 \
    } while (0)

/* Set a double from two 32 bit ints.  */

#define INSERT_WORDS(d,ix0,ix1)                 \
    do {                                        \
        ieee_double_shape_type iw_u;            \
        iw_u.parts.msw = (ix0);                 \
        iw_u.parts.lsw = (ix1);                 \
        (d) = iw_u.value;                       \
    } while (0)

/* Get the more significant 32 bit int from a double.  */

#define GET_HIGH_WORD(i,d)                      \
    do {                                        \
        ieee_double_shape_type gh_u;            \
        gh_u.value = (d);                       \
        (i) = gh_u.parts.msw;                   \
    } while (0)

/* Set the more significant 32 bits of a double from an int.  */

#define SET_HIGH_WORD(d,v)                      \
    do {                                        \
        ieee_double_shape_type sh_u;            \
        sh_u.value = (d);                       \
        sh_u.parts.msw = (v);                   \
        (d) = sh_u.value;                       \
    } while (0)

/* Set the less significant 32 bits of a double from an int.  */

#define SET_LOW_WORD(d,v)                       \
    do {                                        \
        ieee_double_shape_type sl_u;            \
        sl_u.value = (d);                       \
        sl_u.parts.lsw = (v);                   \
        (d) = sl_u.value;                       \
    } while (0)

static const double huge = 1.0e300;

double ceil(double x)
{
    int32_t i0,i1,j0;
    uint32_t i,j;
    EXTRACT_WORDS(i0,i1,x);
    j0 = ((i0>>20)&0x7ff)-0x3ff;
    if(j0<20) {
        if(j0<0) {  /* raise inexact if x != 0 */
        if(huge+x>0.0) {/* return 0*sign(x) if |x|<1 */
            if(i0<0) {i0=0x80000000;i1=0;}
            else if((i0|i1)!=0) { i0=0x3ff00000;i1=0;}
        }
        } else {
        i = (0x000fffff)>>j0;
        if(((i0&i)|i1)==0) return x; /* x is integral */
        if(huge+x>0.0) {    /* raise inexact flag */
            if(i0>0) i0 += (0x00100000)>>j0;
            i0 &= (~i); i1=0;
        }
        }
    } else if (j0>51) {
        if(j0==0x400) return x+x;   /* inf or NaN */
        else return x;      /* x is integral */
    } else {
        i = ((uint32_t)(0xffffffff))>>(j0-20);
        if((i1&i)==0) return x; /* x is integral */
        if(huge+x>0.0) {        /* raise inexact flag */
        if(i0>0) {
            if(j0==20) i0+=1;
            else {
            j = i1 + (1<<(52-j0));
            if(j<i1) i0+=1; /* got a carry */
            i1 = j;
            }
        }
        i1 &= (~i);
        }
    }
    INSERT_WORDS(x,i0,i1);
    return x;
}

static const double
Lg1 = 6.666666666666735130e-01,  /* 3FE55555 55555593 */
Lg2 = 3.999999999940941908e-01,  /* 3FD99999 9997FA04 */
Lg3 = 2.857142874366239149e-01,  /* 3FD24924 94229359 */
Lg4 = 2.222219843214978396e-01,  /* 3FCC71C5 1D8E78AF */
Lg5 = 1.818357216161805012e-01,  /* 3FC74664 96CB03DE */
Lg6 = 1.531383769920937332e-01,  /* 3FC39A09 D078C69F */
Lg7 = 1.479819860511658591e-01;  /* 3FC2F112 DF3E5244 */

/*
 * We always inline k_log1p(), since doing so produces a
 * substantial performance improvement (~40% on amd64).
 */
static inline double k_log1p(double f)
{
    double hfsq,s,z,R,w,t1,t2;

    s = f/(2.0+f);
    z = s*s;
    w = z*z;
    t1= w*(Lg2+w*(Lg4+w*Lg6));
    t2= z*(Lg1+w*(Lg3+w*(Lg5+w*Lg7)));
    R = t2+t1;
    hfsq=0.5*f*f;
    return s*(hfsq+R);
}

static const double
two54      =  1.80143985094819840000e+16, /* 0x43500000, 0x00000000 */
ivln10hi   =  4.34294481878168880939e-01, /* 0x3fdbcb7b, 0x15200000 */
ivln10lo   =  2.50829467116452752298e-11, /* 0x3dbb9438, 0xca9aadd5 */
log10_2hi  =  3.01029995663611771306e-01, /* 0x3FD34413, 0x509F6000 */
log10_2lo  =  3.69423907715893078616e-13; /* 0x3D59FEF3, 0x11F12B36 */

static const double zero   =  0.0;
static volatile double vzero = 0.0;

double log10(double x)
{
    double f,hfsq,hi,lo,r,val_hi,val_lo,w,y,y2;
    int32_t i,k,hx;
    uint32_t lx;

    EXTRACT_WORDS(hx,lx,x);

    k=0;
    if (hx < 0x00100000) {          /* x < 2**-1022  */
        if (((hx&0x7fffffff)|lx)==0)
            return -two54/vzero;        /* log(+-0)=-inf */
        if (hx<0) return (x-x)/zero;    /* log(-#) = NaN */
        k -= 54; x *= two54; /* subnormal number, scale up x */
        GET_HIGH_WORD(hx,x);
    }
    if (hx >= 0x7ff00000) return x+x;
    if (hx == 0x3ff00000 && lx == 0)
        return zero;            /* log(1) = +0 */
    k += (hx>>20)-1023;
    hx &= 0x000fffff;
    i = (hx+0x95f64)&0x100000;
    SET_HIGH_WORD(x,hx|(i^0x3ff00000)); /* normalize x or x/2 */
    k += (i>>20);
    y = (double)k;
    f = x - 1.0;
    hfsq = 0.5*f*f;
    r = k_log1p(f);

    /* See e_log2.c for most details. */
    hi = f - hfsq;
    SET_LOW_WORD(hi,0);
    lo = (f - hi) - hfsq + r;
    val_hi = hi*ivln10hi;
    y2 = y*log10_2hi;
    val_lo = y*log10_2lo + (lo+hi)*ivln10lo + lo*ivln10hi;

    /*
     * Extra precision in for adding y*log10_2hi is not strictly needed
     * since there is no very large cancellation near x = sqrt(2) or
     * x = 1/sqrt(2), but we do it anyway since it costs little on CPUs
     * with some parallelism and it reduces the error for many args.
     */
    w = y2 + val_hi;
    val_lo += (y2 - w) + val_hi;
    val_hi = w;

    return val_lo + val_hi;
}

