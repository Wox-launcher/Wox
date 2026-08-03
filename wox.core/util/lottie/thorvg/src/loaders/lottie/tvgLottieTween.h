/*
 * Copyright (c) 2026 the ThorVG project. All rights reserved.

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

#ifndef _TVG_LOTTIE_TWEEN_
#define _TVG_LOTTIE_TWEEN_

#include "tvgMath.h"
#include "tvgMap.h"
#include "tvgLottieModifier.h"
#include "tvgLottieCommon.h"

struct LottieProperty;

struct LottieTween
{
private:
    union Value {
        PathSet vPath;
        Fill* vFill;
        float vFloat;
        int8_t vInteger;
        Point vScalar;
        Point3 vScalar3;
        RGB32 vColor;
        uint8_t vOpacity;

        Value() : vPath{} {}
        ~Value() {}
    };

    struct ValueSet
    {
        Value from;        // "from" scene data
        Value cur;         // current captured scene data
        uint8_t type = 0;  // 0: general, 1: path, 2: fill
        bool inited = false;
    };

    Map<size_t, ValueSet> data;

    void release()
    {
        // delete the fill values
        for (size_t i = 0; i < data.size; ++i) {
            INLIST_FOREACH(data.buckets[i], item) {
                auto& value = item->val;
                if (value.type == 0) {
                    continue;
                } else if (value.type == 1) {  // path
                    free(value.from.vPath.pts);
                    free(value.from.vPath.cmds);
                    free(value.cur.vPath.pts);
                    free(value.cur.vPath.cmds);
                } else {  // fill
                    delete (value.from.vFill);
                    delete (value.cur.vFill);
                }
            }
        }
        data.clear();
    }

    void copy(PathSet& dst, Point* pts, uint32_t ptsCnt, PathCommand* cmds, uint32_t cmdsCnt)
    {
        // points
        if (dst.ptsCnt != ptsCnt) {
            dst.ptsCnt = ptsCnt;
            dst.pts = tvg::realloc<Point>(dst.pts, sizeof(Point) * ptsCnt);
        }
        memcpy(dst.pts, pts, ptsCnt * sizeof(Point));

        // commands
        if (dst.cmdsCnt != cmdsCnt) {
            dst.cmdsCnt = cmdsCnt;
            dst.cmds = tvg::realloc<PathCommand>(dst.cmds, sizeof(PathCommand) * cmdsCnt);
            if (cmdsCnt > 0) memcpy(dst.cmds, cmds, cmdsCnt * sizeof(PathCommand));  // commands must be same
        }
    }

    void convert(float v, Value& out) { out.vFloat = v; }
    void convert(int8_t v, Value& out) { out.vInteger = v; }
    void convert(const Point& v, Value& out) { out.vScalar = v; }
    void convert(const Point3& v, Value& out) { out.vScalar3 = v; }
    void convert(const RGB32& v, Value& out) { out.vColor = v; }
    void convert(uint8_t v, Value& out) { out.vOpacity = v; }

    void convert(const Value& v, float& out) { out = v.vFloat; }
    void convert(const Value& v, int8_t& out) { out = v.vInteger; }
    void convert(const Value& v, Point& out) { out = v.vScalar; }
    void convert(const Value& v, Point3& out) { out = v.vScalar3; }
    void convert(const Value& v, RGB32& out) { out = v.vColor; }
    void convert(const Value& v, uint8_t& out) { out = v.vOpacity; }

    bool chainswap(size_t key)
    {
        auto set = data.find(key);
        if (!set) return false;
        std::swap(set->val.from, set->val.cur);  // FIXME: copy data
        return true;
    }

public:
    float to = 0.0f;        // used as "to frame number"
    float progress = 0.0f;  // [0 - 1]
    bool active = false;
    bool legacy = false;  // for legacy backward compat

    ~LottieTween()
    {
        release();
    }

    // dynamic
    void on(float to)
    {
        this->to = to;
        active = true;
        legacy = false;

        // initialize the data
        for (size_t i = 0; i < data.size; ++i) {
            INLIST_FOREACH(data.buckets[i], item) {
                item->val.inited = false;
            }
        }
    }

    // legacy
    void on(float to, float progress)
    {
        this->to = to;
        this->progress = progress;
        active = legacy = true;
    }

    void off()
    {
        if (active) {
            active = legacy = false;
            release();
        }
    }

    bool inited(size_t key)
    {
        if (legacy) return false;

        auto set = data.find(key);
        if (!set) return false;
        auto ret = set->val.inited;
        set->val.inited = true;
        return ret;
    }

    // TODO: need a stable composite key?
    size_t key(LottieProperty* prop, float frameNo)
    {
        auto h1 = std::hash<LottieProperty*>{}(prop);
        auto h2 = std::hash<float>{}(frameNo);
        return h1 ^ (h2 + 0x9e3779b9U + (h1 << 6) + (h1 >> 2));
    }

    /************************************************************************/
    /* for General Values                                                   */
    /************************************************************************/

    template<typename T>
    void capture(size_t key, const T& from)
    {
        if (chainswap(key)) return;
        convert(from, data[key].from);
    }

    template<typename T>
    T run(size_t key, const T& to)
    {
        auto& set = data[key];
        T from;
        convert(set.from, from);
        auto cur = tvg::lerp(from, to, progress);
        convert(cur, set.cur);
        return cur;
    }

    /************************************************************************/
    /* for PathSet Value                                                    */
    /************************************************************************/

    void capture(size_t key, RenderPath& from)
    {
        if (chainswap(key)) return;
        auto& set = data[key];
        copy(set.from.vPath, from.pts.data, from.pts.count, from.cmds.data, from.cmds.count);
        set.type = 1;
    }

    // interpolate "from" & "to" then out to "to"
    void run(size_t key, RenderPath& to, RenderPath& out, LottieModifier* modifier)
    {
        auto& set = data[key];
        auto& from = set.from.vPath;
        if (from.ptsCnt != to.pts.count || from.cmdsCnt != to.cmds.count) {
            TVGLOG("LOTTIE", "Tweening has different numbers of path in consecutive frames.");
        }

        auto pivot = out.pts.count;
        auto ptsCnt = std::min(to.pts.count, (uint32_t)from.ptsCnt);

        // interpolate
        if (modifier) {
            for (uint32_t i = 0; i < ptsCnt; ++i) {
                to.pts[i] = tvg::lerp(from.pts[i], to.pts[i], progress);
            }
            modifier->path(to, out, nullptr);
            ptsCnt = out.pts.count - pivot;
        } else {
            for (uint32_t i = 0; i < ptsCnt; ++i) {
                out.pts.push(tvg::lerp(from.pts[i], to.pts[i], progress));
            }
            out.cmds.push(to.cmds);
        }

        copy(set.cur.vPath, out.pts.data + pivot, ptsCnt, from.cmds, from.cmdsCnt);  // capture the current
    }

    /************************************************************************/
    /* for ColorStop Value                                                  */
    /************************************************************************/

    void capture(size_t key, Fill* from)
    {
        if (chainswap(key)) return;
        auto& set = data[key];
        set.from.vFill = from->duplicate();
        set.type = 2;
    }

    // interpolate "from" & "to" then out to "to"
    void run(size_t key, Fill* toFill)
    {
        auto& set = data[key];
        auto fromFill = set.from.vFill;

        // interpolate
        const Fill::ColorStop* from;
        auto fromCnt = fromFill->colorStops(&from);

        const Fill::ColorStop* to;
        auto toCnt = toFill->colorStops(&to);

        if (fromCnt != toCnt) TVGLOG("LOTTIE", "Tweening has different numbers of color data in consecutive frames.");

        for (uint32_t i = 0; i < std::min(fromCnt, toCnt); ++i, ++to, ++from) {
            const_cast<Fill::ColorStop*>(to)->offset = tvg::lerp(from->offset, to->offset, progress);
            const_cast<Fill::ColorStop*>(to)->r = tvg::lerp(from->r, to->r, progress);
            const_cast<Fill::ColorStop*>(to)->g = tvg::lerp(from->g, to->g, progress);
            const_cast<Fill::ColorStop*>(to)->b = tvg::lerp(from->b, to->b, progress);
            const_cast<Fill::ColorStop*>(to)->a = tvg::lerp(from->a, to->a, progress);
        }

        // capture the current
        delete (set.cur.vFill);
        set.cur.vFill = toFill->duplicate();
    }
};

#endif  //_TVG_LOTTIE_TWEEN_
