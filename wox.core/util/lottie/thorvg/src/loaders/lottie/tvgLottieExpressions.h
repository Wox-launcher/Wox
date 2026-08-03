/*
 * Copyright (c) 2024 - 2026 the ThorVG project. All rights reserved.

 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:

 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.

 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

#ifndef _TVG_LOTTIE_EXPRESSIONS_H_
#define _TVG_LOTTIE_EXPRESSIONS_H_

#ifdef THORVG_THREAD_SUPPORT
    #include <thread>
#endif

#include "tvgArray.h"
#include "tvgCommon.h"
#include "tvgLottieCommon.h"

struct LottieExpression;
struct LottieComposition;
struct LottieLayer;
struct LottieModifier;

#ifdef THORVG_LOTTIE_EXPRESSIONS_SUPPORT

#include "jerryscript.h"

struct LottieExpressions
{
    static LottieExpressions* instance();
    static void retrieve(LottieExpressions* instance);

    template<typename Property, typename NumType>
    bool result(float frameNo, NumType& out, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto prop = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            out = (*prop)(frameNo);
        } else {
            out = (NumType) toFloat(bm_rt);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    template<typename Property>
    bool result(float frameNo, uint8_t& out, LottieExpression* exp) {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto prop = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            out = (*prop)(frameNo);
        } else {
            out = (uint8_t)(toFloat(bm_rt) * 2.55f);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    template<typename Property>
    bool result(float frameNo, Point& out, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto prop = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            out = (*prop)(frameNo);
        } else {
            out = toPoint2d(bm_rt);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    template<typename Property>
    bool result(float frameNo, Point3& out, LottieExpression* exp)
    {
        // TODO:
        return false;
    }

    template<typename Property>
    bool result(float frameNo, RGB32& out, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto color = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            out = (*color)(frameNo);
        } else {
            out = toColor(bm_rt);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    template<typename Property>
    bool result(float frameNo, Fill* fill, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto colorStop = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            (*colorStop)(frameNo, fill, this);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    template<typename Property>
    bool result(float frameNo, RenderPath& out, Matrix* transform, LottieModifier* modifier, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (auto pathset = static_cast<Property*>(jerry_object_get_native_ptr(bm_rt, nullptr))) {
            (*pathset)(frameNo, out, transform, nullptr, modifier);
        }
        jerry_value_free(bm_rt);
        return true;
    }

    bool result(float frameNo, TextDocument& doc, LottieExpression* exp)
    {
        auto bm_rt = evaluate(frameNo, exp);
        if (jerry_value_is_undefined(bm_rt)) return false;

        if (jerry_value_is_string(bm_rt)) {
            auto len = jerry_string_length(bm_rt);
            doc.text = tvg::realloc<char>(doc.text, (len + 1) * sizeof(jerry_char_t));
            jerry_string_to_buffer(bm_rt, JERRY_ENCODING_UTF8, (jerry_char_t*)doc.text, len);
            doc.text[len] = '\0';
        }
        jerry_value_free(bm_rt);
        return true;
    }

    void update(float curTime);

private:
    LottieExpressions();
    ~LottieExpressions();

    struct Context
    {
        //global objects, attributes, and methods per local thread instance
        jerry_value_t global;
        jerry_value_t comp;
        jerry_value_t thisComp;
        jerry_value_t thisLayer;
        jerry_value_t thisProperty;
#ifdef THORVG_THREAD_SUPPORT
        jerry_context_t* ctx;
        thread::id tid;
#endif
    };

    Context& context();
    void init(Context& context);
    void clear(Context& context);

    jerry_value_t evaluate(float frameNo, LottieExpression* exp);
    jerry_value_t buildGlobal(Context& context);

    void buildComp(Context& context, LottieComposition* comp, float frameNo, LottieExpression* exp);
    void buildComp(jerry_value_t context, float frameNo, LottieLayer* comp, LottieExpression* exp);
    void buildGlobal(Context& context, float frameNo, LottieExpression* exp);

    Point toPoint2d(jerry_value_t obj);
    RGB32 toColor(jerry_value_t obj);
    float toFloat(jerry_value_t obj);

    Array<Context*> contexts;
};

#else

struct LottieExpressions
{
    static LottieExpressions* instance() { return nullptr; }
    static void retrieve(TVG_UNUSED LottieExpressions*) {}

    template<typename Property, typename NumType> bool result(TVG_UNUSED float, TVG_UNUSED NumType&, TVG_UNUSED LottieExpression*) { return false; }
    template<typename Property> bool result(TVG_UNUSED float, TVG_UNUSED Point&, LottieExpression*) { return false; }
    template<typename Property> bool result(TVG_UNUSED float, TVG_UNUSED RGB32&, TVG_UNUSED LottieExpression*) { return false; }
    template<typename Property> bool result(TVG_UNUSED float, TVG_UNUSED Fill*, TVG_UNUSED LottieExpression*) { return false; }
    template<typename Property> bool result(TVG_UNUSED float, TVG_UNUSED RenderPath&, TVG_UNUSED Matrix*, TVG_UNUSED LottieModifier*, TVG_UNUSED LottieExpression*) { return false; }
    bool result(TVG_UNUSED float, TVG_UNUSED TextDocument& doc, TVG_UNUSED LottieExpression*) { return false; }
    void update(TVG_UNUSED float) {}
};

#endif //THORVG_LOTTIE_EXPRESSIONS_SUPPORT

#endif //_TVG_LOTTIE_EXPRESSIONS_H_