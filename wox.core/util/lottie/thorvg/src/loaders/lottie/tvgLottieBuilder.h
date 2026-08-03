/*
 * Copyright (c) 2023 - 2026 ThorVG project. All rights reserved.

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

#ifndef _TVG_LOTTIE_BUILDER_H_
#define _TVG_LOTTIE_BUILDER_H_

#include "tvgCommon.h"
#include "tvgInlist.h"
#include "tvgShape.h"
#include "tvgLottieExpressions.h"
#include "tvgLottieModifier.h"
#include "tvgLottieTween.h"
#include "thorvg_lottie.h"

struct LottieComposition;

struct RenderRepeater
{
    int cnt;
    Matrix transform;
    float offset;
    Point position;
    Point anchor;
    Point scale;
    float rotation;
    uint8_t startOpacity;
    uint8_t endOpacity;
    bool inorder;
};

struct RenderText
{
    Point cursor{};
    int line = 0, space = 0, idx = 0;
    float lineSpace = 0.0f, totalLineSpace = 0.0f;
    char *p;  //current processing character
    int nChars;
    float scale;
    Scene* textScene;
    Scene* lineScene;
    float capScale, firstMargin;
    LottieTextFollowPath* follow;

    RenderText(LottieText* text, const TextDocument& doc) : p(doc.text), nChars(strlen(p)), scale(doc.size), textScene(Scene::gen()), lineScene(Scene::gen())
    {
    }

    ~RenderText()
    {
        Paint::rel(textScene);
        Paint::rel(lineScene);
    }
};

enum RenderFragment : uint8_t {ByNone = 0, ByFill, ByStroke};

struct RenderContext
{
    INLIST_ITEM(RenderContext);

    Shape* propagator = nullptr;  //for propagating the shape properties excluding paths
    Shape* merging = nullptr;  //merging shapes if possible (if shapes have same properties)
    LottieObject** begin = nullptr; //iteration entry point
    Array<RenderRepeater> repeaters;
    Matrix* transform = nullptr;
    LottieModifier* modifiers = nullptr;
    RenderFragment fragment = ByNone;  //render context has been fragmented
    bool reqFragment = false;  //requirement to fragment the render context

    RenderContext(Shape* propagator)
    {
        to<ShapeImpl>(propagator)->reset();
        propagator->ref();
        this->propagator = propagator;
    }

    ~RenderContext()
    {
        propagator->unref(false);
        delete(transform);
        delete (modifiers);
    }

    RenderContext(const RenderContext& rhs, Shape* propagator, bool mergeable = false) : propagator(propagator)
    {
        if (mergeable) merging = rhs.merging;
        propagator->ref();
        repeaters = rhs.repeaters;
        fragment = rhs.fragment;

        // copy modifiers
        auto m = rhs.modifiers;
        while (m) {
            switch (m->type) {
                case LottieModifier::Type::Roundness: {
                    auto roundness = static_cast<LottieRoundnessModifier*>(m);
                    update(new LottieRoundnessModifier(roundness->r));
                    break;
                }
                case LottieModifier::Type::Offset: {
                    auto offset = static_cast<LottieOffsetModifier*>(m);
                    update(new LottieOffsetModifier(offset->offset, offset->miterLimit, offset->join));
                    break;
                }
                case LottieModifier::Type::PuckerBloat: {
                    auto pucker = static_cast<LottiePuckerBloatModifier*>(m);
                    update(new LottiePuckerBloatModifier(pucker->amount));
                    break;
                }
                case LottieModifier::Type::ZigZag: {
                    auto zigzag = static_cast<LottieZigZagModifier*>(m);
                    update(new LottieZigZagModifier(zigzag->amp, zigzag->freq, zigzag->point));
                    break;
                }
            }
            m = m->next;
        }

        if (rhs.transform) {
            transform = new Matrix;
            *transform = *rhs.transform;
        }
    }

    void update(LottieModifier* next)
    {
        if (modifiers) modifiers = modifiers->decorate(next);
        else modifiers = next;
    }
};

struct AudioResolver
{
    std::function<void(const tvg::LottieAudioResolver& info, void* data)> func;
    void* data = nullptr;
};


struct LottieBuilder
{
    LottieBuilder()
    {
        exps = LottieExpressions::instance();
    }

    ~LottieBuilder()
    {
        LottieExpressions::retrieve(exps);
    }

    bool expressions()
    {
        return exps ? true : false;
    }

    bool update(LottieComposition* comp, float progress);
    void build(LottieComposition* comp);

    const AssetResolver* resolver = nullptr;  //do not free this
    AudioResolver audioResolver;
    LottieTween tween;

private:
    void updateAudio(LottieComposition* comp, LottieLayer* layer, float frameNo);
    void appendRect(LottieRect* rect, Shape* shape, Point& pos, Point& size, float r, bool clockwise, RenderContext* ctx);
    void appendCircle(LottieEllipse* ellipse, Shape* shape, Point& center, Point& radius, bool clockwise, RenderContext* ctx);
    bool fragmented(LottieGroup* parent, LottieObject** child, Inlist<RenderContext>& contexts, RenderContext* ctx, RenderFragment fragment);
    Shape* textShape(LottieText* text, float frameNo, const TextDocument& doc, LottieGlyph* glyph, const RenderText& ctx);

    void updateStrokeEffect(LottieLayer* layer, LottieFxStroke* effect, float frameNo);
    void updateEffect(LottieLayer* layer, float frameNo, uint8_t quality);
    void updateLayer(LottieComposition* comp, Scene* scene, LottieLayer* layer, float frameNo);
    bool updateMatte(LottieComposition* comp, float frameNo, Scene* scene, LottieLayer* layer);
    void updatePrecomp(LottieComposition* comp, LottieLayer* precomp, float frameNo);
    void updatePrecomp(LottieComposition* comp, LottieLayer* precomp, float frameNo, LottieTween& tween);
    void updateSolid(LottieLayer* layer);
    void updateImage(LottieGroup* layer);
    void updateURLFont(LottieLayer* layer, float frameNo, LottieText* text, const TextDocument& doc);
    void updateLocalFont(LottieLayer* layer, float frameNo, LottieText* text, const TextDocument& doc);
    bool updateTextRange(LottieText* text, float frameNo, Shape* shape, const TextDocument& doc, RenderText& ctx);
    void updateText(LottieLayer* layer, float frameNo);
    void updateMasks(LottieLayer* layer, float frameNo);
    void updateTransform(LottieLayer* layer, float frameNo);
    void updateChildren(LottieGroup* parent, float frameNo, Inlist<RenderContext>& contexts);
    void updateGroup(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& pcontexts, RenderContext* ctx);
    void updateTransform(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    bool updateSolidFill(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    bool updateSolidStroke(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    bool updateGradientFill(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    bool updateGradientStroke(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateRect(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateEllipse(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updatePath(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updatePolystar(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateStar(LottiePolyStar* star, float frameNo, Matrix* transform, Shape* merging, RenderContext* ctx, LottieTween& tween, LottieExpressions* exps);
    void updatePolygon(LottieGroup* parent, LottiePolyStar* star, float frameNo, Matrix* transform, Shape* merging, RenderContext* ctx, LottieTween& tween, LottieExpressions* exps);
    void updateTrimpath(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateRepeater(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateRoundedCorner(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateOffsetPath(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updatePuckerBloat(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);
    void updateZigZag(LottieGroup* parent, LottieObject** child, float frameNo, Inlist<RenderContext>& contexts, RenderContext* ctx);

    LottieExpressions* exps;
};

#endif //_TVG_LOTTIE_BUILDER_H