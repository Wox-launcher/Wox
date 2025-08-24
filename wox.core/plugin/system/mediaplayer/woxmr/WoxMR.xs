#define PERL_NO_GET_CONTEXT
#include "EXTERN.h"
#include "perl.h"
#include "XSUB.h"

const char *wox_mr_get_now_playing_json(void);
void wox_mr_free(char *p);

MODULE = WoxMR    PACKAGE = WoxMR

PROTOTYPES: DISABLE

SV*
get_now_playing_json()
    CODE:
    {
        const char *c = wox_mr_get_now_playing_json();
        if (c == NULL) {
            XSRETURN_UNDEF;
        }
        SV *sv = newSVpv(c, 0);
        wox_mr_free((char*)c);
        RETVAL = sv;
    }
    OUTPUT:
        RETVAL

int
toggle()
    CODE:
    {
        RETVAL = wox_mr_toggle();
    }
    OUTPUT:
        RETVAL
