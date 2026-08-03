#ifndef __THORVG_CAPI_H__
#define __THORVG_CAPI_H__

#include <stdint.h>
#include <stdbool.h>

#ifdef TVG_API
    #undef TVG_API
#endif

#define TVG_VERSION_MAJOR 1  // for compile-time checks
#define TVG_VERSION_MINOR 1  // for compile-time checks
#define TVG_VERSION_MICRO 0  // for compile-time checks

#ifndef TVG_STATIC
    #ifdef _WIN32
        #if TVG_BUILD
            #define TVG_API __declspec(dllexport)
        #else
            #define TVG_API __declspec(dllimport)
        #endif
    #elif (defined(__SUNPRO_C)  || defined(__SUNPRO_CC))
        #define TVG_API __global
    #else
        #if (defined(__GNUC__) && __GNUC__ >= 4) || defined(__INTEL_COMPILER)
            #define TVG_API __attribute__ ((visibility("default")))
        #else
            #define TVG_API
        #endif
    #endif
#else
    #define TVG_API
#endif

#ifdef TVG_DEPRECATED
    #undef TVG_DEPRECATED
#endif

#ifdef _WIN32
    #define TVG_DEPRECATED __declspec(deprecated)
#elif __GNUC__ > 3 || (__GNUC__ == 3 && __GNUC_MINOR__ >= 1)
    #define TVG_DEPRECATED __attribute__ ((__deprecated__))
#else
    #define TVG_DEPRECATED
#endif

#ifdef __cplusplus
extern "C"
{
#endif

/**
 * @defgroup ThorVG_CAPI ThorVG_CAPI
 * @brief ThorVG C language binding APIs.
 *
 * \{
 */

/**
 * @brief A structure responsible for managing and drawing graphical elements.
 *
 * It sets up the target buffer, which can be drawn on the screen. It stores the Tvg_Paint objects (Shape, Scene, Picture).
 */
typedef struct _Tvg_Canvas* Tvg_Canvas;

/**
 * @brief A structure representing a graphical element.
 *
 * @warning The TvgPaint objects cannot be shared between Canvases.
 */
typedef struct _Tvg_Paint* Tvg_Paint;

/**
 * @brief A structure representing a gradient fill of a Tvg_Paint object.
 */
typedef struct _Tvg_Gradient* Tvg_Gradient;

/**
 * @brief A structure representing an object that enables to save a Tvg_Paint object into a file.
 */
typedef struct _Tvg_Saver* Tvg_Saver;

/**
 * @brief A structure representing an animation controller object.
 */
typedef struct _Tvg_Animation* Tvg_Animation;

/**
 * @brief A structure representing an object that enables iterating through a scene's descendents.
 */
typedef struct _Tvg_Accessor* Tvg_Accessor;

/**
 * @brief Enumeration specifying the result from the APIs.
 *
 * All ThorVG APIs could potentially return one of the values in the list.
 * Please note that some APIs may additionally specify the reasons that trigger their return values.
 *
 */
typedef enum
{
    TVG_RESULT_SUCCESS = 0,            ///< The value returned in case of a correct request execution.
    TVG_RESULT_INVALID_ARGUMENT,       ///< The value returned in the event of a problem with the arguments given to the API - e.g. empty paths or null pointers.
    TVG_RESULT_INSUFFICIENT_CONDITION, ///< The value returned in case the request cannot be processed - e.g. asking for properties of an object, which does not exist.
    TVG_RESULT_FAILED_ALLOCATION,      ///< The value returned in case of unsuccessful memory allocation.
    TVG_RESULT_MEMORY_CORRUPTION,      ///< The value returned in the event of bad memory handling - e.g. failing in pointer releasing or casting
    TVG_RESULT_NOT_SUPPORTED,          ///< The value returned in case of choosing unsupported engine features(options).
    TVG_RESULT_UNKNOWN = 255           ///< The value returned in all other cases.
} Tvg_Result;

/**
 * @brief A data structure representing a point in two-dimensional space.
 */
typedef struct
{
    float x, y;
} Tvg_Point;

/**
 * @brief A data structure representing a three-dimensional matrix.
 *
 * The elements e11, e12, e21 and e22 represent the rotation matrix, including the scaling factor.
 * The elements e13 and e23 determine the translation of the object along the x and y-axis, respectively.
 * The elements e31 and e32 are set to 0, e33 is set to 1.
 */
typedef struct
{
    float e11, e12, e13;
    float e21, e22, e23;
    float e31, e32, e33;
} Tvg_Matrix;

/**
 * @brief Enumeration specifying the methods of combining the 8-bit color channels into 32-bit color.
 *
 * @ingroup ThorVGCapi_Canvas
 */
typedef enum
{
    TVG_COLORSPACE_ABGR8888 = 0,   ///< The channels are joined in the order: alpha, blue, green, red. Colors are alpha-premultiplied.
    TVG_COLORSPACE_ARGB8888,       ///< The channels are joined in the order: alpha, red, green, blue. Colors are alpha-premultiplied.
    TVG_COLORSPACE_ABGR8888S,      ///< The channels are joined in the order: alpha, blue, green, red. Colors are un-alpha-premultiplied. (since 0.13)
    TVG_COLORSPACE_ARGB8888S,      ///< The channels are joined in the order: alpha, red, green, blue. Colors are un-alpha-premultiplied. (since 0.13)
    TVG_COLORSPACE_GRAYSCALE8,     ///< Single channel, 1 byte per pixel 8-bit grayscale. (since 1.1)
    TVG_COLORSPACE_UNKNOWN = 255,  ///< Unknown channel data. This is reserved for an initial ColorSpace value. (since 1.0)
} Tvg_Colorspace;

/**
 * @brief Enumeration to specify rendering engine behavior.
 *
 * @note The availability or behavior of @c TVG_ENGINE_OPTION_SMART_RENDER may vary depending on platform or backend support.
 *       It attempts to optimize rendering performance by updating only the regions  of the canvas that have
 *       changed between frames (partial redraw). This can be highly effective in scenarios  where most of the
 *       canvas remains static and only small portions are updated—such as simple animations or GUI interactions.
 *       However, in complex scenes where a large portion of the canvas changes frequently (e.g., full-screen animations
 *       or heavy object movements), the overhead of tracking changes and managing update regions may outweigh the benefits,
 *       resulting in decreased performance compared to the default rendering mode. Thus, it is recommended to benchmark
 *       both modes in your specific use case to determine the optimal setting.
 *
 * @ingroup ThorVGCapi_Initializer
 *
 * @since 1.0
 */
typedef enum
{
    TVG_ENGINE_OPTION_NONE = 0,                      /**< No engine options are enabled. This may be used to explicitly disable all optional behaviors. */
    TVG_ENGINE_OPTION_DEFAULT = 1 << 0,              /**< Uses the default rendering mode. */
    TVG_ENGINE_OPTION_SMART_RENDER = 1 << 1,         /**< Enables automatic partial (smart) rendering optimizations. */
    TVG_ENGINE_OPTION_ALIASED = 1 << 2               /**< Disables anti-aliased rendering from the default rendering mode. @note Experimental API */
} Tvg_Engine_Option;

/**
 * @brief Enumeration indicating the method used in the masking of two objects - the target and the source.
 *
 * @ingroup ThorVGCapi_Paint
 */
typedef enum
{
    TVG_MASK_METHOD_NONE = 0,      ///< No Masking is applied.
    TVG_MASK_METHOD_ALPHA,         ///< Alpha Masking using the masking target's pixels as an alpha value.
    TVG_MASK_METHOD_INVERSE_ALPHA, ///< Alpha Masking using the complement to the masking target's pixels as an alpha value.
    TVG_MASK_METHOD_LUMA,          ///< Alpha Masking using the grayscale (0.2126R + 0.7152G + 0.0722*B) of the masking target's pixels. @since 0.9
    TVG_MASK_METHOD_INVERSE_LUMA,  ///< Alpha Masking using the grayscale (0.2126R + 0.7152G + 0.0722*B) of the complement to the masking target's pixels. @since 0.11
    TVG_MASK_METHOD_ADD,           ///< Combines the target and source objects pixels using target alpha. (T * TA) + (S * (255 - TA)) @since 1.0
    TVG_MASK_METHOD_SUBTRACT,      ///< Subtracts the source color from the target color while considering their respective target alpha. (T * TA) - (S * (255 - TA)) @since 1.0
    TVG_MASK_METHOD_INTERSECT,     ///< Computes the result by taking the minimum value between the target alpha and the source alpha and multiplies it with the target color. (T * min(TA, SA)) @since 1.0
    TVG_MASK_METHOD_DIFFERENCE,    ///< Calculates the absolute difference between the target color and the source color multiplied by the complement of the target alpha. abs(T - S * (255 - TA)) @since 1.0
    TVG_MASK_METHOD_LIGHTEN,       ///< Where multiple masks intersect, the highest transparency value is used. @since 1.0
    TVG_MASK_METHOD_DARKEN         ///< Where multiple masks intersect, the lowest transparency value is used. @since 1.0
} Tvg_Mask_Method;

/**
 * @brief Enumeration indicates the method used for blending paint. Please refer to the respective formulas for each method.
 *
 * @ingroup ThorVGCapi_Paint
 *
 * @since 0.15
 */
typedef enum
{
    TVG_BLEND_METHOD_NORMAL = 0,        ///< Perform the alpha blending(default). S if (Sa == 255), otherwise (Sa * S) + (255 - Sa) * D
    TVG_BLEND_METHOD_MULTIPLY,          ///< Takes the RGB channel values from 0 to 255 of each pixel in the top layer and multiples them with the values for the corresponding pixel from the bottom layer. (S * D)
    TVG_BLEND_METHOD_SCREEN,            ///< The values of the pixels in the two layers are inverted, multiplied, and then inverted again. (S + D) - (S * D)
    TVG_BLEND_METHOD_OVERLAY,           ///< Combines Multiply and Screen blend modes. (2 * S * D) if (2 * D < Da), otherwise (Sa * Da) - 2 * (Da - S) * (Sa - D)
    TVG_BLEND_METHOD_DARKEN,            ///< Creates a pixel that retains the smallest components of the top and bottom layer pixels. min(S, D)
    TVG_BLEND_METHOD_LIGHTEN,           ///< Only has the opposite action of Darken Only. max(S, D)
    TVG_BLEND_METHOD_COLORDODGE,        ///< Divides the bottom layer by the inverted top layer. D / (255 - S)
    TVG_BLEND_METHOD_COLORBURN,         ///< Divides the inverted bottom layer by the top layer, and then inverts the result. 255 - (255 - D) / S
    TVG_BLEND_METHOD_HARDLIGHT,         ///< The same as Overlay but with the color roles reversed. (2 * S * D) if (S < Sa), otherwise (Sa * Da) - 2 * (Da - S) * (Sa - D)
    TVG_BLEND_METHOD_SOFTLIGHT,         ///< Darkens or lightens the colors, depending on the source color value. If S <= 0.5: D - (1 - 2 * S) * D * (1 - D), otherwise: D + (2 * S - 1) * (G(D) - D), where G(D) = ((16 * D - 12) * D + 4) * D if D <= 0.25, otherwise sqrt(D).
    TVG_BLEND_METHOD_DIFFERENCE,        ///< Subtracts the bottom layer from the top layer or the other way around, to always get a non-negative value. (S - D) if (S > D), otherwise (D - S)
    TVG_BLEND_METHOD_EXCLUSION,         ///< The result is twice the product of the top and bottom layers, subtracted from their sum. s + d - (2 * s * d)
    TVG_BLEND_METHOD_HUE,               ///< Uses the hue of the source and the saturation and luminosity of the destination. @since 1.0
    TVG_BLEND_METHOD_SATURATION,        ///< Uses the saturation of the source and the hue and luminosity of the destination. @since 1.0
    TVG_BLEND_METHOD_COLOR,             ///< Uses the hue and saturation of the source and the luminosity of the destination. @since 1.0
    TVG_BLEND_METHOD_LUMINOSITY,        ///< Uses the luminosity of the source and the hue and saturation of the destination. @since 1.0
    TVG_BLEND_METHOD_ADD,               ///< Simply adds pixel values of one layer with the other. (S + D)
    TVG_BLEND_METHOD_COMPOSITION = 255  ///< Used for intermediate composition. @since 1.0
} Tvg_Blend_Method;

/**
 * @brief Enumeration indicating the ThorVG object type value.
 *
 * ThorVG's drawing objects can return object type values, allowing you to identify the specific type of each object.
 *
 * @ingroup ThorVGCapi_Paint
 *
 * @see tvg_paint_get_type()
 * @see tvg_gradient_get_type()
 *
 * @since 1.0
 */
typedef enum
{
    TVG_TYPE_UNDEF = 0,        ///< Undefined type.
    TVG_TYPE_SHAPE,            ///< A shape type paint.
    TVG_TYPE_SCENE,            ///< A scene type paint.
    TVG_TYPE_PICTURE,          ///< A picture type paint.
    TVG_TYPE_TEXT,             ///< A text type paint.
    TVG_TYPE_LINEAR_GRAD = 10, ///< A linear gradient type.
    TVG_TYPE_RADIAL_GRAD       ///< A radial gradient type.
} Tvg_Type;

/**
 * @addtogroup ThorVGCapi_Shape
 * \{
 */

/**
 * @brief Enumeration specifying the values of the path commands accepted by ThorVG.
 */
typedef uint8_t Tvg_Path_Command;

enum
{
    TVG_PATH_COMMAND_CLOSE = 0,    ///< Ends the current sub-path and connects it with its initial point - corresponds to Z command in the svg path commands.
    TVG_PATH_COMMAND_MOVE_TO,      ///< Sets a new initial point of the sub-path and a new current point - corresponds to M command in the svg path commands.
    TVG_PATH_COMMAND_LINE_TO,      ///< Draws a line from the current point to the given point and sets a new value of the current point - corresponds to L command in the svg path commands.
    TVG_PATH_COMMAND_CUBIC_TO      ///< Draws a cubic Bezier curve from the current point to the given point using two given control points and sets a new value of the current point - corresponds to C command in the svg path commands.
};

/**
 * @brief Enumeration determining the ending type of a stroke in the open sub-paths.
 */
typedef enum {
    TVG_STROKE_CAP_BUTT = 0, ///< The stroke ends exactly at each of the two endpoints of a sub-path. For zero length sub-paths no stroke is rendered.
    TVG_STROKE_CAP_ROUND,    ///< The stroke is extended in both endpoints of a sub-path by a half circle, with a radius equal to the half of a stroke width. For zero length sub-paths a full circle is rendered.
    TVG_STROKE_CAP_SQUARE    ///< The stroke is extended in both endpoints of a sub-path by a rectangle, with the width equal to the stroke width and the length equal to the half of the stroke width. For zero length sub-paths the square is rendered with the size of the stroke width.
} Tvg_Stroke_Cap;

/**
 * @brief Enumeration specifying how to fill the area outside the gradient bounds.
 */
typedef enum
{
    TVG_STROKE_JOIN_MITER = 0, ///< The outer corner of the joined path segments is spiked. The spike is created by extension beyond the join point of the outer edges of the stroke until they intersect. In case the extension goes beyond the limit, the join style is converted to the Bevel style.
    TVG_STROKE_JOIN_ROUND,     ///< The outer corner of the joined path segments is rounded. The circular region is centered at the join point.
    TVG_STROKE_JOIN_BEVEL      ///< The outer corner of the joined path segments is bevelled at the join point. The triangular region of the corner is enclosed by a straight line between the outer corners of each stroke.
} Tvg_Stroke_Join;

/**
 * @brief Enumeration specifying how to fill the area outside the gradient bounds.
 */
typedef enum
{
    TVG_STROKE_FILL_PAD = 0, ///< The remaining area is filled with the closest stop color.
    TVG_STROKE_FILL_REFLECT, ///< The gradient pattern is reflected outside the gradient area until the expected region is filled.
    TVG_STROKE_FILL_REPEAT   ///< The gradient pattern is repeated continuously beyond the gradient area until the expected region is filled.
} Tvg_Stroke_Fill;

/**
 * @brief Enumeration specifying the algorithm used to establish which parts of the shape are treated as the inside of the shape.
 */
typedef enum
{
    TVG_FILL_RULE_NON_ZERO = 0, ///< A line from the point to a location outside the shape is drawn. The intersections of the line with the path segment of the shape are counted. Starting from zero, if the path segment of the shape crosses the line clockwise, one is added, otherwise one is subtracted. If the resulting sum is non zero, the point is inside the shape.
    TVG_FILL_RULE_EVEN_ODD     ///< A line from the point to a location outside the shape is drawn and its intersections with the path segments of the shape are counted. If the number of intersections is an odd number, the point is inside the shape.
} Tvg_Fill_Rule;

/** \} */   // end addtogroup ThorVGCapi_Shape

/**
 * @addtogroup ThorVGCapi_Gradient
 * \{
 */

/**
 * @brief A data structure storing the information about the color and its relative position inside the gradient bounds.
 */
typedef struct
{
    float offset; /**< The relative position of the color. */
    uint8_t r;    /**< The red color channel value in the range [0 ~ 255]. */
    uint8_t g;    /**< The green color channel value in the range [0 ~ 255]. */
    uint8_t b;    /**< The blue color channel value in the range [0 ~ 255]. */
    uint8_t a;    /**< The alpha channel value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque. */
} Tvg_Color_Stop;

/** \} */   // end addtogroup ThorVGCapi_Gradient

/**
 * @addtogroup ThorVGCapi_Text
 * \{
 */

/**
 * @brief A data structure storing the information about the color and its relative position inside the gradient bounds.
 */
typedef enum
{
    TVG_TEXT_WRAP_NONE = 0,      ///< Do not wrap text. Text is rendered on a single line and may overflow the bounding area.
    TVG_TEXT_WRAP_CHARACTER,     ///< Wrap at the character level. If a word cannot fit, it is broken into individual characters to fit the line.
    TVG_TEXT_WRAP_WORD,          ///< Wrap at the word level. Words that do not fit are moved to the next line.
    TVG_TEXT_WRAP_SMART,         ///< Smart choose wrapping method: word wrap first, falling back to character wrap if a word does not fit.
    TVG_TEXT_WRAP_ELLIPSIS,      ///< Truncate overflowing text and append an ellipsis ("...") at the end. Typically used for single-line labels.
    TVG_TEXT_WRAP_HYPHENATION    ///< Reserved. No Support.
} Tvg_Text_Wrap;

/** \} */  // end addtogroup ThorVGCapi_Text

/**
 * @addtogroup ThorVGCapi_Picture
 * \{
 */

/**
 * @brief Defines the image filtering method used during image scaling or transformation.
 *
 * @since 1.1
 */
typedef enum
{
    TVG_FILTER_METHOD_BILINEAR = 0,  ///< Smooth interpolation using surrounding pixels for higher quality.
    TVG_FILTER_METHOD_NEAREST        ///< Fast filtering using nearest-neighbor sampling.
} Tvg_Filter_Method;

/**
 * @brief Describes the font metrics of a text object.
 *
 * Provides the basic vertical layout metrics used for text rendering,
 * such as ascent, descent, and line spacing (linegap).
 *
 * @see tvg_text_get_text_metrics()
 * @note Experimental API
 */
typedef struct
{
    float ascent;   ///< Distance from the baseline to the top of the highest glyph (usually positive).
    float descent;  ///< Distance from the baseline to the bottom of the lowest glyph (usually negative, as in TTF).
    float linegap;  ///< Additional spacing recommended between lines (leading).
    float advance;  ///< The total vertical advance between lines of text: ascent - descent + linegap (i.e., ascent + |descent| + linegap when descent is negative).
} Tvg_Text_Metrics;

/**
 * @brief Describes the layout metrics of a glyph.
 *
 * Provides the basic layout metrics used for positioning an individual glyph,
 * including its advance along the baseline direction, bearing relative to the
 * inline axis origin, and its bounding box in local glyph space.
 *
 * The advance value represents the distance the pen position moves along the
 * baseline (inline direction), regardless of whether the text is laid out
 * horizontally or vertically.
 *
 * The bounding box is defined in the glyph’s local coordinate space and is
 * independent of any layout direction or transformation.
 *
 * @see tvg_text_get_glyph_metrics()
 * @note Experimental API
 */
typedef struct
{
    float advance;    ///< The advance distance along the baseline (inline) direction.
    float bearing;    ///< The bearing from the origin to the glyph’s visible bound along the inline-start direction.
    Tvg_Point min;    ///< The minimum point of the glyph bounding box in local space.
    Tvg_Point max;    ///< The maximum point of the glyph bounding box in local space.
} Tvg_Glyph_Metrics;

/**
 * @brief Callback function type for resolving external assets.
 *
 * This callback is invoked when a Picture requires an external asset
 * (such as an image or font resource). Implementations should load the asset
 * into the given @p paint object.
 *
 * @param[in] paint The target paint object where the resolved asset will be loaded.
 * @param[in] src   The source path, identifier, or URI of the asset to be resolved.
 * @param[in] data  User-provided custom data passed to the callback for context.
 *
 * @return @c true if the asset was successfully resolved and loaded into @p paint, otherwise @c false.
 *
 * @see tvg_picture_set_asset_resolver()
 *
 * @note Experimental API
 */
typedef bool (*Tvg_Picture_Asset_Resolver)(Tvg_Paint paint, const char* src, void* data);

/** \} */   // end addtogroup ThorVGCapi_Picture

/**
 * @defgroup ThorVGCapi_Initializer Initializer
 * @brief A module enabling initialization and termination of the TVG engines.
 *
 * \{
 */

/************************************************************************/
/* Engine API                                                           */
/************************************************************************/
/**
 * @brief Initializes the ThorVG engine.
 *
 * ThorVG requires an active runtime environment to operate.
 * Internally, it utilizes a task scheduler to efficiently parallelize rendering operations.
 * You can specify the number of worker threads using the @p threads parameter.
 * During initialization, ThorVG will spawn the specified number of threads.
 *
 * @param[in] threads The number of worker threads to create. A value of zero indicates that only the main thread will be used.
 *
 * @note The initializer uses internal reference counting to track multiple calls.
 *       The number of threads is fixed on the first call to tvg_engine_init() and cannot be changed in subsequent calls.
 * @see tvg_engine_term()
 */
TVG_API Tvg_Result tvg_engine_init(unsigned threads);

/**
 * @brief Terminates the ThorVG engine.
 *
 * Cleans up resources and stops any internal threads initialized by tvg_engine_init().
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION Returned if there is nothing to terminate (e.g., tvg_engine_init() was not called).
 *
 * @note The initializer maintains a reference count for safe repeated use. Only the final call to tvg_engine_term() will fully shut down the engine.
 * @see tvg_engine_init()
 */
TVG_API Tvg_Result tvg_engine_term(void);

/**
 * @brief Retrieves the version of the TVG engine.
 *
 * @param[out] major A major version number.
 * @param[out] minor A minor version number.
 * @param[out] micro A micro version number.
 * @param[out] version The version of the engine in the format major.minor.micro, or a @p nullptr in case of an internal error.
 *
 * @retval TVG_RESULT_SUCCESS.
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_engine_version(uint32_t* major, uint32_t* minor, uint32_t* micro, const char** version);

/** \} */   // end defgroup ThorVGCapi_Initializer

/**
 * @defgroup ThorVGCapi_Canvas Canvas
 * @brief A module for managing and drawing graphical elements.
 *
 * A canvas is an entity responsible for drawing the target. It sets up the drawing engine and the buffer, which can be drawn on the screen. It also manages given Paint objects.
 *
 * @note A Canvas behavior depends on the raster engine though the final content of the buffer is expected to be identical.
 * @warning The paint objects belonging to one Canvas can't be shared among multiple Canvases.
 * \{
 */

/**
 * @defgroup ThorVGCapi_SwCanvas SwCanvas
 * @ingroup ThorVGCapi_Canvas
 *
 * @brief A module for rendering the graphical elements using the software engine.
 *
 * \{
 */

/************************************************************************/
/* SwCanvas API                                                         */
/************************************************************************/

/**
 * @brief Creates a new Software Canvas object with optional rendering engine settings.
 *
 * This method generates a software canvas instance that can be used for drawing vector graphics.
 * It accepts an optional parameter @p op to choose between different rendering engine behaviors.
 *
 * @param[in] op The rendering engine option.
 *
 * @return A new canvas object.
 *
 * @see enum Tvg_Engine_Option
 */
TVG_API Tvg_Canvas tvg_swcanvas_create(Tvg_Engine_Option op);

/**
 * @brief Sets the buffer used in the rasterization process and defines the used colorspace.
 *
 * For optimisation reasons TVG does not allocate memory for the output buffer on its own.
 * The buffer of a desirable size should be allocated and owned by the caller.
 *
 * @param[in] canvas The canvas object managing the @p buffer.
 * @param[in] buffer The allocated memory block of the size @p stride x @p h.
 * @param[in] stride The stride of the raster image - in most cases same value as @p w.
 * @param[in] w The width of the raster image.
 * @param[in] h The height of the raster image.
 * @param[in] cs The colorspace value defining the way the 32-bits colors should be read/written.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENTS An invalid canvas or buffer pointer passed or one of the @p stride, @p w or @p h being zero.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the canvas is performing rendering. Please ensure the canvas is synced.
 * @retval TVG_RESULT_NOT_SUPPORTED The software engine is not supported.
 *
 * @warning Do not access @p buffer during tvg_canvas_draw() - tvg_canvas_sync(). It should not be accessed while the engine is writing on it.
 *
 * @note Currently, only @c TVG_COLORSPACE_ABGR8888, @c TVG_COLORSPACE_ARGB8888, @c TVG_COLORSPACE_ABGR8888S, and @c TVG_COLORSPACE_ARGB8888S are supported for @p cs.
 * @see Tvg_Colorspace
 */
TVG_API Tvg_Result tvg_swcanvas_set_target(Tvg_Canvas canvas, uint32_t* buffer, uint32_t stride, uint32_t w, uint32_t h, Tvg_Colorspace cs);

/** \} */   // end defgroup ThorVGCapi_SwCanvas

/**
 * @defgroup ThorVGCapi_GlCanvas SwCanvas
 * @ingroup ThorVGCapi_Canvas
 *
 * @brief A module for rendering the graphical elements using the opengl engine.
 *
 * \{
 */

/************************************************************************/
/* GlCanvas API                                                         */
/************************************************************************/

/**
 * @brief Creates a new OpenGL/ES Canvas object with optional rendering engine settings.
 *
 * This method generates a OpenGL/ES canvas instance that can be used for drawing vector graphics.
 * It accepts an optional parameter @p op to choose between different rendering engine behaviors.
 *
 * @param[in] op The rendering engine option.
 *
 * @return A new canvas object.
 *
 * @note Currently, it does not support @c TVG_ENGINE_OPTION_SMART_RENDER. The request will be ignored.
 *
 * @see enum Tvg_Engine_Option
 *
 * @since 1.0
 */
TVG_API Tvg_Canvas tvg_glcanvas_create(Tvg_Engine_Option op);

/**
 * @brief Sets the drawing target for rasterization.
 *
 * This function specifies the drawing target where the rasterization will occur. It can target
 * a specific framebuffer object (FBO) or the main surface.
 *
 * @param[in] display The platform-specific display handle (EGLDisplay for EGL). Set @c nullptr for other systems.
 * @param[in] surface The platform-specific surface handle (EGLSurface for EGL, HDC for WGL). Set @c nullptr for other systems.
 * @param[in] context The OpenGL context to be used for rendering on this canvas.
 * @param[in] id The GL target ID, usually indicating the FBO ID. A value of @c 0 specifies the main surface.
 * @param[in] w The width (in pixels) of the raster image.
 * @param[in] h The height (in pixels) of the raster image.
 * @param[in] cs Specifies how the pixel values should be interpreted. Currently, it only allows @c TVG_COLORSPACE_ABGR8888S as @c GL_RGBA8.
 *
 * @note If @p display and @p surface are not provided, the ThorVG GL engine assumes that
 *       the appropriate OpenGL context is already current and will not attempt to bind a new one.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the canvas is currently rendering.
 *         Ensure that @ref tvg_canvas_sync() has been called before setting a new target.
 * @retval TVG_RESULT_NOT_SUPPORTED In case the gl engine is not supported.
 *
 * @see tvg_canvas_sync()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_glcanvas_set_target(Tvg_Canvas canvas, void* display, void* surface, void* context, int32_t id, uint32_t w, uint32_t h, Tvg_Colorspace cs);

/** \} */   // end defgroup ThorVGCapi_GlCanvas

/**
 * @defgroup ThorVGCapi_WgCanvas WgCanvas
 * @ingroup ThorVGCapi_Canvas
 *
 * @brief A module for rendering the graphical elements using the webgpu engine.
 *
 * \{
 */

/************************************************************************/
/* WgCanvas API                                                         */
/************************************************************************/


/**
 * @brief Encapsulates the WebGPU context required for rendering.
 *
 * This structure contains the WebGPU objects used to initialize the rendering backend.
 *
 * @note Experimental API
 */
typedef struct {
        void* instance;  // WGPUInstance, context for all other wgpu objects.
        void* adapter;   // WGPUAdapter, the adapter associated with the rendering device.
        void* device;    // WGPUDevice, a desired handle for the wgpu device.
} Tvg_WgContext;

/**
 * @brief Creates a new WebGPU Canvas object with optional rendering engine settings.
 *
 * This method generates a WebGPU canvas instance that can be used for drawing vector graphics.
 * It accepts an optional parameter @p op to choose between different rendering engine behaviors.
 *
 * @param[in] op The rendering engine option.
 *
 * @return A new canvas object.
 *
 * @note Currently, it does not support @c TVG_ENGINE_OPTION_SMART_RENDER. The request will be ignored.
 *
 * @see enum Tvg_Engine_Option
 *
 * @since 1.0
 */
TVG_API Tvg_Canvas tvg_wgcanvas_create(Tvg_Engine_Option op);

/**
 * @brief Sets the drawing target for the rasterization.
 *
 * @param[in] device WGPUDevice, a desired handle for the wgpu device.
 * @param[in] instance WGPUInstance, context for all other wgpu objects.
 * @param[in] target Either WGPUSurface or WGPUTexture, serving as handles to a presentable surface or texture.
 * @param[in] w The width of the target.
 * @param[in] h The height of the target.
 * @param[in] cs Specifies how the pixel values should be interpreted. Currently, it allows @c TVG_COLORSPACE_ABGR8888 and @c TVG_COLORSPACE_ABGR8888S.
 * @param[in] type @c 0: surface, @c 1: texture are used as presentable target.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the canvas is performing rendering. Please ensure the canvas is synced.
 * @retval TVG_RESULT_NOT_SUPPORTED In case the wg engine is not supported.
 *
 * @warning Regardless of the value of @p cs, this target API uses the default alpha mode.
 *
 * @see tvg_wgcanvas_set_target_with_context()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_wgcanvas_set_target(Tvg_Canvas canvas, void* device, void* instance, void* target, uint32_t w, uint32_t h, Tvg_Colorspace cs, int type);

/**
 * @brief Sets the drawing target for the rasterization.
 *
 * @param[in] context Tvg_WgContext context.
 * @param[in] target Either WGPUSurface or WGPUTexture, serving as handles to a presentable surface or texture.
 * @param[in] w The width of the target.
 * @param[in] h The height of the target.
 * @param[in] cs Specifies how the pixel values should be interpreted. Currently, it allows @c TVG_COLORSPACE_ABGR8888 and @c TVG_COLORSPACE_ABGR8888S.
 * @param[in] type @c 0: surface, @c 1: texture are used as presentable target.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the canvas is performing rendering. Please ensure the canvas is synced.
 * @retval TVG_RESULT_NOT_SUPPORTED In case the wg engine is not supported.
 *
 * @note Experimental API
 */
TVG_API Tvg_Result tvg_wgcanvas_set_target_with_context(Tvg_Canvas canvas, const Tvg_WgContext* context, void* target, uint32_t w, uint32_t h, Tvg_Colorspace cs, int type);

/** \} */   // end defgroup ThorVGCapi_WgCanvas

/************************************************************************/
/* Common Canvas API                                                    */
/************************************************************************/
/**
 * @brief Clears the canvas internal data, releases all paints stored by the canvas and destroys the canvas object itself.
 *
 * @param[in] canvas The canvas object to be destroyed.
 *
 */
TVG_API Tvg_Result tvg_canvas_destroy(Tvg_Canvas canvas);

/**
 * @brief Adds a paint object to the canvas for rendering.
 *
 * Adds the specified paint into the canvas root scene. Only paints added to
 * the canvas are considered rendering targets. The canvas retains the paint
 * object until it is explicitly removed via tvg_canvas_remove().
 *
 * @param[in] canvas A handle to the canvas that will manage the paint object.
 * @param[in] paint  A handle to the paint object to be rendered.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the canvas is not in a valid state to accept new paints.
 *
 * @note Ownership of the @p paint object is transferred to the canvas upon
 *       successful addition. To retain ownership, call @ref tvg_paint_ref()
 *       before adding it to the canvas.
 * @note The rendering order of paint objects follows the order in which they are
 *       added to the canvas. If layering is required, ensure paints are added in
 *       the desired order.
 *
 * @see tvg_canvas_insert()
 * @see tvg_canvas_remove()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_canvas_add(Tvg_Canvas canvas, Tvg_Paint paint);

/**
 * @brief Inserts a paint object into the canvas root scene.
 *
 * Inserts a paint object into the root scene of the specified canvas. If the
 * @p at parameter is provided, the paint object is inserted immediately before
 * the specified paint in the root scene. If @p at is @c nullptr, the paint object
 * is appended to the end of the root scene.
 *
 * @param[in] canvas A handle to the canvas that will manage the paint object.
 *                   This parameter must be a valid canvas handle.
 * @param[in] target A handle to the paint object to be inserted into the root scene.
 *                   This parameter must not be @c nullptr.
 * @param[in] at     A handle to an existing paint object in the root scene before
 *                   which @p target will be inserted. If @c nullptr, @p target is
 *                   appended to the end of the root scene.
 *
 * @note Ownership of the @p target object is transferred to the canvas upon
 *       successful addition. To retain ownership, call @ref tvg_paint_ref()
 *       before adding it to the canvas.
 * @note The rendering order of paint objects follows their order in the root
 *       scene. If layering is required, ensure paints are inserted in the
 *       desired order.
 *
 * @see tvg_canvas_add()
 * @see tvg_canvas_remove()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_canvas_insert(Tvg_Canvas canvas, Tvg_Paint target, Tvg_Paint at);

/**
 * @brief Removes a paint object from the root scene.
 *
 * This function removes a specified paint object from the root scene. If no paint
 * object is specified (i.e., the default @c nullptr is used), the function
 * performs to clear all paints from the scene.
 *
 * @param[in] canvas A canvas object to remove the @p paint.
 * @param[in] paint The paint object to be removed from the root scene.
 *                  If @c nullptr, remove all the paints from the root scene.
 *
 * @see tvg_canvas_add()
 * @see tvg_canvas_insert()
 * @since 1.0
 */
TVG_API Tvg_Result tvg_canvas_remove(Tvg_Canvas canvas, Tvg_Paint paint);

/**
 * @brief Requests the canvas to update modified paint objects in preparation for rendering.
 *
 * This function triggers an internal update for all paint instances that have been modified
 * since the last update. It ensures that the canvas state is ready for accurate rendering.
 *
 * @param[in] canvas The canvas object to be updated.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION The canvas is not properly prepared.
 *         This may occur if the canvas target has not been set or if the update is called during drawing.
 *         Call tvg_canvas_sync() before trying.
 *
 * @note Only paint objects that have been changed will be processed.
 * @note If the canvas is configured with multiple threads, the update may be performed asynchronously.
 *
 * @see tvg_canvas_sync()
 */
TVG_API Tvg_Result tvg_canvas_update(Tvg_Canvas canvas);

/**
 * @brief Requests the canvas to render the Paint objects.
 *
 * @param[in] canvas The canvas object containing elements to be drawn.
 * @param[in] clear If @c true, clears the target buffer to zero before drawing.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION The canvas is not properly prepared.
 *         This may occur if the canvas target has not been set or if the update is called during drawing.
 *         without calling tvg_canvas_sync() in between.
 *
 * @note Clearing the buffer is unnecessary if the canvas will be fully covered
 *       with opaque content. Skipping the clear can improve performance.
 * @note Drawing may be performed asynchronously if the thread count is greater than zero.
 *       To ensure the drawing process is complete, call sync() afterwards.
 * @note If the canvas has not been updated prior to tvg_canvas_draw(), it may implicitly perform tvg_canvas_update()
 *
 * @see tvg_canvas_sync()
 * @see tvg_canvas_update()
 */
TVG_API Tvg_Result tvg_canvas_draw(Tvg_Canvas canvas, bool clear);

/**
 * @brief Guarantees that drawing task is finished.
 *
 * @param[in] canvas The canvas object containing elements which were drawn.
 *
 * The Canvas rendering can be performed asynchronously. To make sure that rendering is finished,
 * the tvg_canvas_sync() must be called after the tvg_canvas_draw() regardless of threading.
 *
 *
 * @see tvg_canvas_draw()
 */
TVG_API Tvg_Result tvg_canvas_sync(Tvg_Canvas canvas);

/**
 * @brief Sets the drawing region of the canvas.
 *
 * This function defines a rectangular area of the canvas to be used for drawing operations.
 * The specified viewport clips rendering output to the boundaries of that rectangle.
 *
 * Please note that changing the viewport is only allowed at the beginning of the rendering sequence—that is, after calling tvg_canvas_sync().
 *
 * @param[in] canvas The canvas object containing elements which were drawn.
 * @param[in] x The x-coordinate of the upper-left corner of the rectangle.
 * @param[in] y The y-coordinate of the upper-left corner of the rectangle.
 * @param[in] w The width of the rectangle.
 * @param[in] h The height of the rectangle.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the canvas is not in a synced state.
 *
 * @see tvg_canvas_sync()
 * @see tvg_swcanvas_set_target()
 * @see tvg_glcanvas_set_target()
 * @see tvg_wgcanvas_set_target()
 *
 * @warning Changing the viewport is not allowed after calling tvg_canvas_add(),
 *          tvg_canvas_remove(), tvg_canvas_update(), or tvg_canvas_draw().
 *
 * @note When the target is reset, the viewport will also be reset to match the target size.
 * @since 0.15
 */
TVG_API Tvg_Result tvg_canvas_set_viewport(Tvg_Canvas canvas, int32_t x, int32_t y, int32_t w, int32_t h);

/** \} */   // end defgroup ThorVGCapi_Canvas

/**
 * @defgroup ThorVGCapi_Paint Paint
 * @brief A module for managing graphical elements. It enables duplication, transformation and composition.
 *
 * \{
 */

/************************************************************************/
/* Paint API                                                            */
/************************************************************************/

/**
 * @brief Safely releases a Tv_Paint object.
 *
 * This is the counterpart to the `new()` API, and releases the given Paint object safely, 
 * handling @c nullptr and managing ownership properly.
 *
 * @param[in] paint A paint object to release.
 */
TVG_API Tvg_Result tvg_paint_rel(Tvg_Paint paint);

/**
 * @brief Increment the reference count for the Tvg_Paint object.
 *
 * This method increases the reference count of Tvg_Paint object, allowing shared ownership and control over its lifetime.
 *
 * @param[in] paint The paint object to increase the reference count.
 *
 * @return The updated reference count after the increment by 1.
 *
 * @warning Please ensure that each call to tvg_paint_ref() is paired with a corresponding call to tvg_paint_unref() to prevent a dangling instance.
 *
 * @see tvg_paint_unref()
 * @see tvg_paint_get_ref()
 *
 * @since 1.0
 */
TVG_API uint16_t tvg_paint_ref(Tvg_Paint paint);

/**
 * @brief Decrement the reference count for the Tvg_Paint object.
 *
 * This method decreases the reference count of the Tvg_Paint object by 1.
 * If the reference count reaches zero and the @p free flag is set to true, the instance is automatically deleted.
 *
 * @param[in] paint The paint object to decrease the reference count.
 * @param[in] free Flag indicating whether to delete the Paint instance when the reference count reaches zero.
 *
 * @return The updated reference count after the decrement.
 *
 * @see tvg_paint_ref()
 * @see tvg_paint_get_ref()
 *
 * @since 1.0
 */
TVG_API uint16_t tvg_paint_unref(Tvg_Paint paint, bool free);

/**
 * @brief Retrieve the current reference count of the Tvg_Paint object.
 *
 * This method provides the current reference count, allowing the user to check the shared ownership state of the Tvg_Paint object.
 *
 * @param[in] paint The paint object to return the reference count.
 *
 * @return The current reference count of the Tvg_Paint object.
 *
 * @see tvg_paint_ref()
 * @see tvg_paint_unref()
 *
 * @since 1.0
 */
TVG_API uint16_t tvg_paint_get_ref(const Tvg_Paint paint);

/**
 * @brief Sets the visibility of the Paint object.
 *
 * This is useful for selectively excluding paint objects during rendering.
 *
 * @param[in] paint The paint object to set the visibility status.
 * @param[in] on A boolean flag indicating visibility. The default is @c true.
 *               @c true, the object will be rendered by the engine.
 *               @c false, the object will be excluded from the drawing process.
 *
 * @note An invisible object is not considered inactive—it may still participate
 *       in internal update processing if its properties are updated, but it will not
 *       be taken into account for the final drawing output. To completely deactivate
 *       a paint object, remove it from the canvas.
 *
 * @see tvg_paint_get_visible()
 * @see tvg_canvas_remove()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_paint_set_visible(Tvg_Paint paint, bool visible);

/**
 * @brief Gets the current visibility status of the Paint object.
 *
 * @param[in] paint The paint object to return the visibility status.
 *
 * @return true if the object is visible and will be rendered.
 *         false if the object is hidden and will not be rendered.
 *
 * @see tvg_paint_set_visible()
 *
 * @since 1.0
 */
TVG_API bool tvg_paint_get_visible(const Tvg_Paint paint);

/**
 * @brief Gets the ID of the Paint object.
 *
 * @param[in] paint The paint object whose ID will be returned.
 *
 * @return The ID of the paint object, or 0 if the ID is not set.
 *
 * @see tvg_picture_get_paint()
 * @see tvg_accessor_generate_id()
 * @see tvg_paint_set_id()
 *
 * @since 1.1
 */
TVG_API uint32_t tvg_paint_get_id(const Tvg_Paint paint);

/**
 * @brief Sets the ID of the Paint object.
 *
 * The ID is used to specify a paint instance in a scene.
 *
 * @param[in] paint The paint object whose ID will be set.
 * @param[in] id The ID to assign to the paint object.
 *
 * @see tvg_picture_get_paint()
 * @see tvg_accessor_generate_id()
 * @see tvg_paint_get_id()
 *
 * @since 1.1
 */
TVG_API Tvg_Result tvg_paint_set_id(Tvg_Paint paint, uint32_t id);

/**
 * @brief Scales the given Tvg_Paint object by the given factor.
 *
 * @param[in] paint The paint object to be scaled.
 * @param[in] factor The value of the scaling factor. The default value is 1.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION in case a custom transform is applied.
 *
 * @see tvg_paint_set_transform()
 */
TVG_API Tvg_Result tvg_paint_scale(Tvg_Paint paint, float factor);

/**
 * @brief Rotates the given Tvg_Paint by the given angle.
 *
 * The angle in measured clockwise from the horizontal axis.
 * The rotational axis passes through the point on the object with zero coordinates.
 *
 * @param[in] paint The paint object to be rotated.
 * @param[in] degree The value of the rotation angle in degrees.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION in case a custom transform is applied.
 *
 * @see tvg_paint_set_transform()
 */
TVG_API Tvg_Result tvg_paint_rotate(Tvg_Paint paint, float degree);

/**
 * @brief Moves the given Tvg_Paint in a two-dimensional space.
 *
 * The origin of the coordinate system is in the upper-left corner of the canvas.
 * The horizontal and vertical axes point to the right and down, respectively.
 *
 * @param[in] paint The paint object to be shifted.
 * @param[in] x The value of the horizontal shift.
 * @param[in] y The value of the vertical shift.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION in case a custom transform is applied.
 *
 * @see tvg_paint_set_transform()
 */
TVG_API Tvg_Result tvg_paint_translate(Tvg_Paint paint, float x, float y);

/**
 * @brief Transforms the given Tvg_Paint using the augmented transformation matrix.
 *
 * The augmented matrix of the transformation is expected to be given.
 *
 * @param[in] paint The paint object to be transformed.
 * @param[in] m The 3x3 augmented matrix.
 * 
 */
TVG_API Tvg_Result tvg_paint_set_transform(Tvg_Paint paint, const Tvg_Matrix* m);

/**
 * @brief Gets the matrix of the affine transformation of the given Tvg_Paint object.
 *
 * In case no transformation was applied, the identity matrix is returned.
 *
 * @param[in] paint The paint object of which to get the transformation matrix.
 * @param[out] m The 3x3 augmented matrix.
 *
 */
TVG_API Tvg_Result tvg_paint_get_transform(Tvg_Paint paint, Tvg_Matrix* m);

/**
 * @brief Sets the opacity of the given Tvg_Paint.
 *
 * @param[in] paint The paint object of which the opacity value is to be set.
 * @param[in] opacity The opacity value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque.
 *
 * @note Setting the opacity with this API may require multiple renderings using a composition. It is recommended to avoid changing the opacity if possible.
 */
TVG_API Tvg_Result tvg_paint_set_opacity(Tvg_Paint paint, uint8_t opacity);

/**
 * @brief Gets the opacity of the given Tvg_Paint.
 *
 * @param[in] paint The paint object of which to get the opacity value.
 * @param[out] opacity The opacity value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque.
 *
 */
TVG_API Tvg_Result tvg_paint_get_opacity(const Tvg_Paint paint, uint8_t* opacity);

/**
 * @brief Duplicates the given Tvg_Paint object.
 *
 * Creates a new object and sets its all properties as in the original object.
 *
 * @param[in] paint The paint object to be copied.
 *
 * @return A copied Tvg_Paint object if succeed, @c nullptr otherwise.
 */
TVG_API Tvg_Paint tvg_paint_duplicate(Tvg_Paint paint);

/**
 * @brief Checks whether a given region intersects the filled area of the paint.
 *
 * This function determines whether the specified rectangular region—defined by (`x`, `y`, `w`, `h`)—
 * intersects the geometric fill region of the paint object.
 *
 * This is useful for hit-testing purposes, such as detecting whether a user interaction (e.g., touch or click)
 * occurs within a painted region.
 *
 * The paint must be updated in a Canvas beforehand—typically after the Canvas has been
 * drawn and synchronized.
 *
 * @param[in] paint The paint object to be tested.
 * @param[in] x The x-coordinate of the top-left corner of the test region.
 * @param[in] y The y-coordinate of the top-left corner of the test region.
 * @param[in] w The width of the region to test. Must be greater than 0.
 * @param[in] h The height of the region to test. Must be greater than 0.
 *
 * @return @c true if any part of the region intersects the filled area; otherwise, @c false.
 *
 * @note To test a single point, set the region size to w = 1, h = 1.
 * @note For efficiency, an AABB (axis-aligned bounding box) test is performed internally before precise hit detection.
 * @note This test does not take into account the results of blending or masking.
 * @note This test does take into account hidden paints as well.
 * @see tvg_paint_set_visible()
 * @since 1.0
 */
TVG_API bool tvg_paint_intersects(Tvg_Paint paint, int32_t x, int32_t y, int32_t w, int32_t h);

/**
 * @brief Checks whether a given region intersects the filled area of the paint.
 *
 * This function determines whether the specified rectangular region—defined by (`x`, `y`, `w`, `h`)—
 * intersects the geometric fill region of the paint object.
 *
 * This is useful for hit-testing purposes, such as detecting whether a user interaction (e.g., touch or click)
 * occurs within a painted region.
 *
 * The paint must be updated in a Canvas beforehand—typically after the Canvas has been
 * drawn and synchronized.
 *
 * @param[in] paint The paint object to be tested.
 * @param[in] x The x-coordinate of the top-left corner of the test region.
 * @param[in] y The y-coordinate of the top-left corner of the test region.
 * @param[in] w The width of the region to test. Must be greater than 0.
 * @param[in] h The height of the region to test. Must be greater than 0.
 * @param[in] visibleOnly If @c true, hidden paints are excluded from the intersection test.
 *
 * @return @c true if any part of the region intersects the filled area; otherwise, @c false.
 *
 * @note To test a single point, set the region size to w = 1, h = 1.
 * @note This test does not take into account the results of blending or masking.
 *
 * @see tvg_paint_set_visible()
 * @since Experimental API
 */
TVG_API bool tvg_paint_intersects_region(Tvg_Paint paint, int32_t x, int32_t y, int32_t w, int32_t h, bool visibleOnly);

/**
 * @brief Retrieves the axis-aligned bounding box (AABB) of the paint object in canvas space.
 *
 * Returns the bounding box of the paint as an axis-aligned bounding box (AABB), with all relevant transformations applied.
 * The returned values @p x, @p y, @p w, @p h, may have invalid if the operation fails. Thus, please check the retval.
 *
 * This bounding box can be used to determine the actual rendered area of the object on the canvas,
 * for purposes such as hit-testing, culling, or layout calculations.
 *
 * @param[in] paint The paint object of which to get the bounds.
 * @param[out] x The x-coordinate of the upper-left corner of the bounding box.
 * @param[out] y The y-coordinate of the upper-left corner of the bounding box.
 * @param[out] w The width of the bounding box.
 * @param[out] h The height of the bounding box.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If it failed to compute the bounding box (mostly due to invalid path information).
 *
 * @see tvg_paint_get_obb()
 * @see tvg_canvas_update()
 */
TVG_API Tvg_Result tvg_paint_get_aabb(Tvg_Paint paint, float* x, float* y, float* w, float* h);

/**
 * @brief Retrieves the object-oriented bounding box (OBB) of the paint object in canvas space.
 *
 * This function returns the bounding box of the paint, as an oriented bounding box (OBB) after transformations are applied.
 * The returned values @p pt4 may have invalid if the operation fails. Thus, please check the retval.
 *
 * This bounding box can be used to obtain the transformed bounding region in canvas space
 * by taking the geometry's axis-aligned bounding box (AABB) in the object's local coordinate space
 * and applying the object's transformations.
 *
 * @param[in] paint The paint object of which to get the bounds.
 * @param[out] pt4 An array of four points representing the bounding box. The array size must be 4.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If it failed to compute the bounding box (mostly due to invalid path information).
 *
 * @see tvg_paint_get_aabb()
 * @see tvg_canvas_update()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_paint_get_obb(Tvg_Paint paint, Tvg_Point* pt4);

/**
 * @brief Sets the masking target object and the masking method.
 *
 * @param[in] paint The source object of the masking.
 * @param[in] target The target object of the masking.
 * @param[in] method The method used to mask the source object with the target.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the target has already belonged to another paint.
 *
 */
TVG_API Tvg_Result tvg_paint_set_mask_method(Tvg_Paint paint, Tvg_Paint target, Tvg_Mask_Method method);

/**
 * @brief Gets the masking target object and the masking method.
 *
 * @param[in] paint The source object of the masking.
 * @param[out] target The target object of the masking.
 * @param[out] method The method used to mask the source object with the target.
 *
 */
TVG_API Tvg_Result tvg_paint_get_mask_method(const Tvg_Paint paint, const Tvg_Paint target, Tvg_Mask_Method* method);

/**
 * @brief Clip the drawing region of the paint object.
 *
 * This function restricts the drawing area of the paint object to the specified shape's paths.
 *
 * @param[in] paint The target object of the clipping.
 * @param[in] clipper The shape object as the clipper.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the target has already belonged to another paint.
 * @retval TVG_RESULT_NOT_SUPPORTED If the @p clipper type is not Shape.
 *
 * @see tvg_paint_get_clip()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_paint_set_clip(Tvg_Paint paint, Tvg_Paint clipper);

/**
 * @brief Get the clipper shape of the paint object.
 *
 * This function returns the clipper that has been previously set to this paint object.
 *
 * @return The shape object used as the clipper, or @c nullptr if no clipper is set.
 *
 * @see tvg_paint_set_clip()
 *
 * @since 1.0
 */
TVG_API Tvg_Paint tvg_paint_get_clip(const Tvg_Paint paint);

/**
 * @brief Retrieves the parent paint object.
 *
 * This function returns the parent object if the current paint
 * belongs to one. Otherwise, it returns @c nullptr.
 *
 * @param[in] paint The paint object of which to get the scene.
 *
 * @return The parent object if available, otherwise @c nullptr.
 *
 * @see tvg_scene_add()
 * @see tvg_canvas_add()
 *
 * @since 1.0
*/
TVG_API Tvg_Paint tvg_paint_get_parent(const Tvg_Paint paint);

/**
 * @brief Gets the unique value of the paint instance indicating the instance type.
 *
 * @param[in] paint The paint object of which to get the type value.
 * @param[out] type The unique type of the paint instance type.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_paint_get_type(const Tvg_Paint paint, Tvg_Type* type);

/**
 * @brief Sets the blending method for the paint object.
 *
 * The blending feature allows you to combine colors to create visually appealing effects, including transparency, lighting, shading, and color mixing, among others.
 * its process involves the combination of colors or images from the source paint object with the destination (the lower layer image) using blending operations.
 * The blending operation is determined by the chosen @p BlendMethod, which specifies how the colors or images are combined.
 *
 * @param[in] paint The paint object of which to set the blend method.
 * @param[in] method The blending method to be set.
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_paint_set_blend_method(Tvg_Paint paint, Tvg_Blend_Method method);

/** \} */   // end defgroup ThorVGCapi_Paint

/**
 * @defgroup ThorVGCapi_Shape Shape
 *
 * @brief A module for managing two-dimensional figures and their properties.
 *
 * A shape has three major properties: shape outline, stroking, filling. The outline in the shape is retained as the path.
 * Path can be composed by accumulating primitive commands such as tvg_shape_move_to(), tvg_shape_line_to(), tvg_shape_cubic_to() or complete shape interfaces such as tvg_shape_append_rect(), tvg_shape_append_circle(), etc.
 * Path can consists of sub-paths. One sub-path is determined by a close command.
 *
 * The stroke of a shape is an optional property in case the shape needs to be represented with/without the outline borders.
 * It's efficient since the shape path and the stroking path can be shared with each other. It's also convenient when controlling both in one context.
 *
 * \{
 */

/************************************************************************/
/* Shape API                                                            */
/************************************************************************/

/**
 * @brief Creates a new Shape object.
 *
 * This function allocates and returns a new Shape instance.
 * To properly destroy the Shape object, use @ref tvg_paint_rel().
 *
 * @return The newly created Shape object.
 *
 * @see tvg_paint_rel()
 */
TVG_API Tvg_Paint tvg_shape_new(void);

/**
 * @brief Resets the shape path properties.
 *
 * The color, the fill and the stroke properties are retained.
 *
 * @param[in] paint The shape object.
 *
 * @note The memory, where the path data is stored, is not deallocated at this stage for caching effect.
 */
TVG_API Tvg_Result tvg_shape_reset(Tvg_Paint paint);

/**
 * @brief Sets the initial point of the sub-path.
 *
 * The value of the current point is set to the given point.
 *
 * @param[in] paint The shape object.
 * @param[in] x The horizontal coordinate of the initial point of the sub-path.
 * @param[in] y The vertical coordinate of the initial point of the sub-path.
 *
 */
TVG_API Tvg_Result tvg_shape_move_to(Tvg_Paint paint, float x, float y);

/**
 * @brief Adds a new point to the sub-path, which results in drawing a line from the current point to the given end-point.
 *
 * The value of the current point is set to the given end-point.
 *
 * @param[in] paint The shape object.
 * @param[in] x The horizontal coordinate of the end-point of the line.
 * @param[in] y The vertical coordinate of the end-point of the line.
 *
 * @note In case this is the first command in the path, it corresponds to the tvg_shape_move_to() call.
 */
TVG_API Tvg_Result tvg_shape_line_to(Tvg_Paint paint, float x, float y);

/**
 * @brief Adds new points to the sub-path, which results in drawing a cubic Bezier curve.
 *
 * The Bezier curve starts at the current point and ends at the given end-point (@p x, @p y). Two control points (@p cx1, @p cy1) and (@p cx2, @p cy2) are used to determine the shape of the curve.
 * The value of the current point is set to the given end-point.
 *
 * @param[in] paint The shape object.
 * @param[in] cx1 The horizontal coordinate of the 1st control point.
 * @param[in] cy1 The vertical coordinate of the 1st control point.
 * @param[in] cx2 The horizontal coordinate of the 2nd control point.
 * @param[in] cy2 The vertical coordinate of the 2nd control point.
 * @param[in] x The horizontal coordinate of the endpoint of the curve.
 * @param[in] y The vertical coordinate of the endpoint of the curve.
 *
 * @note In case this is the first command in the path, no data from the path are rendered.
 */
TVG_API Tvg_Result tvg_shape_cubic_to(Tvg_Paint paint, float cx1, float cy1, float cx2, float cy2, float x, float y);

/**
 * @brief Closes the current sub-path by drawing a line from the current point to the initial point of the sub-path.
 *
 * The value of the current point is set to the initial point of the closed sub-path.
 *
 * @param[in] paint The shape object.
 *
 * @note In case the sub-path does not contain any points, this function has no effect.
 */
TVG_API Tvg_Result tvg_shape_close(Tvg_Paint paint);

/**
 * @brief Appends a rectangle to the path.
 *
 * The rectangle with rounded corners can be achieved by setting non-zero values to @p rx and @p ry arguments.
 * The @p rx and @p ry values specify the radii of the ellipse defining the rounding of the corners.
 *
 * The position of the rectangle is specified by the coordinates of its upper-left corner -  @p x and @p y arguments.
 *
 * The rectangle is treated as a new sub-path - it is not connected with the previous sub-path.
 *
 * The value of the current point is set to (@p x + @p w, @p y + @p ry) - in case @p ry is greater
 * than @p h/2 the current point is set to (@p x + @p w, @p y + @p h/2).
 *
 * @param[in] paint The shape object.
 * @param[in] x The horizontal coordinate of the upper-left corner of the rectangle.
 * @param[in] y The vertical coordinate of the upper-left corner of the rectangle.
 * @param[in] w The width of the rectangle.
 * @param[in] h The height of the rectangle.
 * @param[in] rx The x-axis radius of the ellipse defining the rounded corners of the rectangle.
 * @param[in] ry The y-axis radius of the ellipse defining the rounded corners of the rectangle.
 * @param[in] cw Specifies the path direction: @c true for clockwise, @c false for counterclockwise.
 *
 * @note For @p rx and @p ry greater than or equal to the half of @p w and the half of @p h, respectively, the shape become an ellipse.
 */
TVG_API Tvg_Result tvg_shape_append_rect(Tvg_Paint paint, float x, float y, float w, float h, float rx, float ry, bool cw);

/**
 * @brief Appends an ellipse to the path.
 *
 * The position of the ellipse is specified by the coordinates of its center - @p cx and @p cy arguments.
 *
 * The ellipse is treated as a new sub-path - it is not connected with the previous sub-path.
 *
 * The value of the current point is set to (@p cx, @p cy - @p ry).
 *
 * @param[in] paint The shape object.
 * @param[in] cx The horizontal coordinate of the center of the ellipse.
 * @param[in] cy The vertical coordinate of the center of the ellipse.
 * @param[in] rx The x-axis radius of the ellipse.
 * @param[in] ry The y-axis radius of the ellipse.
 * @param[in] cw Specifies the path direction: @c true for clockwise, @c false for counterclockwise.
 *
 */
TVG_API Tvg_Result tvg_shape_append_circle(Tvg_Paint paint, float cx, float cy, float rx, float ry, bool cw);

/**
 * @brief Appends a given sub-path to the path.
 *
 * The current point value is set to the last point from the sub-path.
 * For each command from the @p cmds array, an appropriate number of points in @p pts array should be specified.
 * If the number of points in the @p pts array is different than the number required by the @p cmds array, the shape with this sub-path will not be displayed on the screen.
 *
 * @param[in] paint The shape object.
 * @param[in] cmds The array of the commands in the sub-path.
 * @param[in] cmdCnt The length of the @p cmds array.
 * @param[in] pts The array of the two-dimensional points.
 * @param[in] ptsCnt The length of the @p pts array.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT A @c nullptr passed as the argument or @p cmdCnt or @p ptsCnt equal to zero.
 */
TVG_API Tvg_Result tvg_shape_append_path(Tvg_Paint paint, const Tvg_Path_Command* cmds, uint32_t cmdCnt, const Tvg_Point* pts, uint32_t ptsCnt);

/**
 * @brief Retrieves the current path data of the shape.
 *
 * This function provides access to the shape's path data, including the commands
 * and points that define the path.
 *
 * @param[out] cmds Pointer to the array of commands representing the path.
 *                  Can be @c nullptr if this information is not needed.
 * @param[out] cmdsCnt Pointer to the variable that receives the number of commands in the @p cmds array.
 *                     Can be @c nullptr if this information is not needed.
 * @param[out] pts Pointer to the array of two-dimensional points that define the path.
 *                 Can be @c nullptr if this information is not needed.
 * @param[out] ptsCnt Pointer to the variable that receives the number of points in the @p pts array.
 *                    Can be @c nullptr if this information is not needed.
 *
 * @note If any of the arguments are @c nullptr, that value will be ignored.
 */
TVG_API Tvg_Result tvg_shape_get_path(const Tvg_Paint paint, const Tvg_Path_Command** cmds, uint32_t* cmdsCnt, const Tvg_Point** pts, uint32_t* ptsCnt);

/**
 * @brief Sets the stroke width for the path.
 *
 * This function defines the thickness of the stroke applied to all figures
 * in the path object. A stroke is the outline drawn along the edges of the
 * path's geometry.
 *
 * @param[in] paint The shape object.
 * @param[in] width The width of the stroke in pixels. Must be positive value. (The default is 0)
 *
 * @note A value of @p width 0 disables the stroke.
 *
 * @see tvg_shape_set_stroke_color()
 */
TVG_API Tvg_Result tvg_shape_set_stroke_width(Tvg_Paint paint, float width);

/**
 * @brief Gets the shape's stroke width.
 *
 * @param[in] paint The shape object.
 * @param[out] width The stroke width.
 *
 */
TVG_API Tvg_Result tvg_shape_get_stroke_width(const Tvg_Paint paint, float* width);

/**
 * @brief Sets the shape's stroke color.
 *
 * @param[in] paint The shape object.
 * @param[in] r The red color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] g The green color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] b The blue color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] a The alpha channel value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque.
 *
 * @note If the stroke width is 0 (default), the stroke will not be visible regardless of the color.
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 *
 * @see tvg_shape_set_stroke_width()
 * @see tvg_shape_set_stroke_gradient()
 */
TVG_API Tvg_Result tvg_shape_set_stroke_color(Tvg_Paint paint, uint8_t r, uint8_t g, uint8_t b, uint8_t a);

/**
 * @brief Gets the shape's stroke color.
 *
 * @param[in] paint The shape object.
 * @param[out] r The red color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] g The green color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] b The blue color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] a The alpha channel value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION No stroke was set.
 */
TVG_API Tvg_Result tvg_shape_get_stroke_color(const Tvg_Paint paint, uint8_t* r, uint8_t* g, uint8_t* b, uint8_t* a);

/**
 * @brief Sets the gradient fill of the stroke for all of the figures from the path.
 *
 * @param[in] paint The shape object.
 * @param[in] grad The gradient fill.
 *
 * @retval TVG_RESULT_MEMORY_CORRUPTION An invalid gradient object or an error with accessing it.
 *
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 *
 * @see tvg_shape_set_stroke_color()
 */
TVG_API Tvg_Result tvg_shape_set_stroke_gradient(Tvg_Paint paint, Tvg_Gradient grad);

/**
 * @brief Gets the gradient fill of the shape's stroke.
 *
 * The function does not allocate any memory.
 *
 * @param[in] paint The shape object.
 * @param[out] grad The gradient fill.
 *
 */
TVG_API Tvg_Result tvg_shape_get_stroke_gradient(const Tvg_Paint paint, Tvg_Gradient* grad);

/**
 * @brief Sets the shape's stroke dash pattern.
 *
 * @param[in] paint The shape object.
 * @param[in] dashPattern An array of alternating dash and gap lengths.
 * @param[in] cnt The size of the @p dashPattern array.
 * @param[in] offset The shift of the starting point within the repeating dash pattern, from which the pattern begins to be applied.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT In case @p dashPattern is @c nullptr and @p cnt > 0 or @p dashPattern is not @c nullptr and @p cnt is zero.
 *
 * @note To reset the stroke dash pattern, pass @c nullptr to @p dashPattern and zero to @p cnt.
 * @note Values of @p dashPattern less than zero are treated as zero.
 * @note If all values in the @p dashPattern are equal to or less than 0, the dash is ignored.
 * @note If the @p dashPattern contains an odd number of elements, the sequence is repeated in the same
 * order to form an even-length pattern, preserving the alternation of dashes and gaps.
 * @since 1.0
 */
TVG_API Tvg_Result tvg_shape_set_stroke_dash(Tvg_Paint paint, const float* dashPattern, uint32_t cnt, float offset);

/**
 * @brief Gets the dash pattern of the stroke.
 *
 * The function does not allocate any memory.
 *
 * @param[in] paint The shape object.
 * @param[out] dashPattern The array of consecutive pair values of the dash length and the gap length.
 * @param[out] cnt The size of the @p dashPattern array.
 * @param[out] offset The shift of the starting point within the repeating dash pattern.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_shape_get_stroke_dash(const Tvg_Paint paint, const float** dashPattern, uint32_t* cnt, float* offset);

/**
 * @brief Sets the cap style used for stroking the path.
 *
 * The cap style specifies the shape to be used at the end of the open stroked sub-paths.
 *
 * @param[in] paint The shape object.
 * @param[in] cap The cap style value. The default value is @c TVG_STROKE_CAP_SQUARE.
 *
 */
TVG_API Tvg_Result tvg_shape_set_stroke_cap(Tvg_Paint paint, Tvg_Stroke_Cap cap);

/**
 * @brief Gets the stroke cap style used for stroking the path.
 *
 * @param[in] paint The shape object.
 * @param[out] cap The cap style value.
 *
 */
TVG_API Tvg_Result tvg_shape_get_stroke_cap(const Tvg_Paint paint, Tvg_Stroke_Cap* cap);

/**
 * @brief Sets the join style for stroked path segments.
 *
 * @param[in] paint The shape object.
 * @param[in] join The join style value. The default value is @c TVG_STROKE_JOIN_BEVEL.
 *
 */
TVG_API Tvg_Result tvg_shape_set_stroke_join(Tvg_Paint paint, Tvg_Stroke_Join join);

/**
 * @brief The function gets the stroke join method
 *
 * @param[in] paint The shape object.
 * @param[out] join The join style value.
 *
 */
TVG_API Tvg_Result tvg_shape_get_stroke_join(const Tvg_Paint paint, Tvg_Stroke_Join* join);

/**
 * @brief Sets the stroke miterlimit.
 *
 * @param[in] paint The shape object.
 * @param[in] miterlimit The miterlimit imposes a limit on the extent of the stroke join when the @c TVG_STROKE_JOIN_MITER join style is set. The default value is 4.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT Unsupported @p miterlimit values (less than zero).
 *
 * @since 0.11
 */
TVG_API Tvg_Result tvg_shape_set_stroke_miterlimit(Tvg_Paint paint, float miterlimit);

/**
 * @brief The function gets the stroke miterlimit.
 *
 * @param[in] paint The shape object.
 * @param[out] miterlimit The stroke miterlimit.
 *
 *
 * @since 0.11
 */
TVG_API Tvg_Result tvg_shape_get_stroke_miterlimit(const Tvg_Paint paint, float* miterlimit);

/**
 * @brief Sets the trim of the shape along the defined path segment, allowing control over which part of the shape is visible.
 *
 * If the values of the arguments @p begin and @p end exceed the 0-1 range, they are wrapped around in a manner similar to angle wrapping, effectively treating the range as circular.
 *
 * @param[in] paint The shape object.
 * @param[in] begin Specifies the start of the segment to display along the path.
 * @param[in] end Specifies the end of the segment to display along the path.
 * @param[in] simultaneous Determines how to trim multiple paths within a single shape. If set to @c true (default), trimming is applied simultaneously to all paths;
 *                         Otherwise, all paths are treated as a single entity with a combined length equal to the sum of their individual lengths and are trimmed as such.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_shape_set_trimpath(Tvg_Paint paint, float begin, float end, bool simultaneous);

/**
 * @brief Sets the shape's solid color.
 *
 * The parts of the shape defined as inner are colored.
 *
 * @param[in] paint The shape object.
 * @param[in] r The red color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] g The green color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] b The blue color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] a The alpha channel value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque. The default value is 0.
 *
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 * @see tvg_shape_set_fill_rule()
 */
TVG_API Tvg_Result tvg_shape_set_fill_color(Tvg_Paint paint, uint8_t r, uint8_t g, uint8_t b, uint8_t a);

/**
 * @brief Gets the shape's solid color.
 *
 * @param[in] paint The shape object.
 * @param[out] r The red color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] g The green color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] b The blue color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[out] a The alpha channel value in the range [0 ~ 255], where 0 is completely transparent and 255 is opaque. The default value is 0.
 *
 */
TVG_API Tvg_Result tvg_shape_get_fill_color(const Tvg_Paint paint, uint8_t* r, uint8_t* g, uint8_t* b, uint8_t* a);

/**
 * @brief Sets the fill rule for the shape.
 *
 * Specifies how the interior of the shape is determined when its path intersects itself.
 * The default fill rule is @c TVG_FILL_RULE_NON_ZERO.
 *
 * @param[in] paint The shape object.
 * @param[in] rule The fill rule to apply to the shape.
 *
 */
TVG_API Tvg_Result tvg_shape_set_fill_rule(Tvg_Paint paint, Tvg_Fill_Rule rule);

/**
 * @brief Retrieves the current fill rule used by the shape.
 *
 * This function returns the fill rule, which determines how the interior 
 * regions of the shape are calculated when it overlaps itself.
 *
 * @param[in] paint The shape object.
 * @param[out] rule The current Tvg_Fill_Rule value of the shape.
 *
 */
TVG_API Tvg_Result tvg_shape_get_fill_rule(const Tvg_Paint paint, Tvg_Fill_Rule* rule);

/**
 * @brief Sets the rendering order of the stroke and the fill.
 *
 * @param[in] paint The shape object.
 * @param[in] strokeFirst If @c true the stroke is rendered before the fill, otherwise the stroke is rendered as the second one (the default option).
 *
 * @since 0.10
 */
TVG_API Tvg_Result tvg_shape_set_paint_order(Tvg_Paint paint, bool strokeFirst);

/**
 * @brief Sets the gradient fill for all of the figures from the path.
 *
 * The parts of the shape defined as inner are filled.
 *
 * @param[in] paint The shape object.
 * @param[in] grad The gradient fill.
 *
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 * @see tvg_shape_set_fill_rule()
 */
TVG_API Tvg_Result tvg_shape_set_gradient(Tvg_Paint paint, Tvg_Gradient grad);

/**
 * @brief Gets the gradient fill of the shape.
 *
 * The function does not allocate any data.
 *
 * @param[in] paint The shape object.
 * @param[out] grad The gradient fill.
 *
 */
TVG_API Tvg_Result tvg_shape_get_gradient(const Tvg_Paint paint, Tvg_Gradient* grad);

/** \} */   // end defgroup ThorVGCapi_Shape

/**
 * @defgroup ThorVGCapi_Gradient Gradient
 * @brief A module managing the gradient fill of objects.
 *
 * The module enables to set and to get the gradient colors and their arrangement inside the gradient bounds,
 * to specify the gradient bounds and the gradient behavior in case the area defined by the gradient bounds
 * is smaller than the area to be filled.
 *
 * \{
 */

/************************************************************************/
/* Gradient API                                                         */
/************************************************************************/
/**
 * @brief Creates a new linear gradient object.
 *
 * @return A new linear gradient object.
 */
TVG_API Tvg_Gradient tvg_linear_gradient_new(void);

/**
 * @brief Creates a new radial gradient object.
 *
 * @return A new radial gradient object.
 */
TVG_API Tvg_Gradient tvg_radial_gradient_new(void);

/**
 * @brief Sets the linear gradient bounds.
 *
 * The bounds of the linear gradient are defined as a surface constrained by two parallel lines crossing
 * the given points (@p x1, @p y1) and (@p x2, @p y2), respectively. Both lines are perpendicular to the line linking
 * (@p x1, @p y1) and (@p x2, @p y2).
 *
 * @param[in] grad The Tvg_Gradient object of which bounds are to be set.
 * @param[in] x1 The horizontal coordinate of the first point used to determine the gradient bounds.
 * @param[in] y1 The vertical coordinate of the first point used to determine the gradient bounds.
 * @param[in] x2 The horizontal coordinate of the second point used to determine the gradient bounds.
 * @param[in] y2 The vertical coordinate of the second point used to determine the gradient bounds.
 *
 * @note In case the first and the second points are equal, an object is filled with a single color using the last color specified in the tvg_gradient_set_color_stops().
 * @see tvg_gradient_set_color_stops()
 */
TVG_API Tvg_Result tvg_linear_gradient_set(Tvg_Gradient grad, float x1, float y1, float x2, float y2);

/**
 * @brief Gets the linear gradient bounds.
 *
 * The bounds of the linear gradient are defined as a surface constrained by two parallel lines crossing
 * the given points (@p x1, @p y1) and (@p x2, @p y2), respectively. Both lines are perpendicular to the line linking
 * (@p x1, @p y1) and (@p x2, @p y2).
 *
 * @param[in] grad The Tvg_Gradient object of which to get the bounds.
 * @param[out] x1 The horizontal coordinate of the first point used to determine the gradient bounds.
 * @param[out] y1 The vertical coordinate of the first point used to determine the gradient bounds.
 * @param[out] x2 The horizontal coordinate of the second point used to determine the gradient bounds.
 * @param[out] y2 The vertical coordinate of the second point used to determine the gradient bounds.
 *
 */
TVG_API Tvg_Result tvg_linear_gradient_get(Tvg_Gradient grad, float* x1, float* y1, float* x2, float* y2);

/**
 * @brief Sets the radial gradient attributes.
 *
 * The radial gradient is defined by the end circle with a center (@p cx, @p cy) and a radius @p r and
 * the start circle with a center/focal point (@p fx, @p fy) and a radius @p fr.
 * The gradient will be rendered such that the gradient stop at an offset of 100% aligns with the edge of the end circle
 * and the stop at an offset of 0% aligns with the edge of the start circle.
 *
 * @param[in] grad The Tvg_Gradient object of which bounds are to be set.
 * @param[in] cx The horizontal coordinate of the center of the end circle.
 * @param[in] cy The vertical coordinate of the center of the end circle.
 * @param[in] r The radius of the end circle.
 * @param[in] fx The horizontal coordinate of the center of the start circle.
 * @param[in] fy The vertical coordinate of the center of the start circle.
 * @param[in] fr The radius of the start circle.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT The radius @p r or @p fr value is negative.
 *
 * @note In case the radius @p r is zero, an object is filled with a single color using the last color specified in the specified in the tvg_gradient_set_color_stops().
 * @note In case the focal point (@p fx and @p fy) lies outside the end circle, it is projected onto the edge of the end circle.
 * @note If the start circle doesn't fully fit inside the end circle (after possible repositioning), the @p fr is reduced accordingly.
 * @note By manipulating the position and size of the focal point, a wide range of visual effects can be achieved, such as directing
 *       the gradient focus towards a specific edge or enhancing the depth and complexity of shading patterns.
 *       If a focal effect is not desired, simply align the focal point (@p fx and @p fy) with the center of the end circle (@p cx and @p cy)
 *       and set the radius (@p fr) to zero. This will result in a uniform gradient without any focal variations.
 *
 * @see tvg_gradient_set_color_stops()
 */
TVG_API Tvg_Result tvg_radial_gradient_set(Tvg_Gradient grad, float cx, float cy, float r, float fx, float fy, float fr);

/**
 * @brief The function gets radial gradient attributes.
 *
 * @param[in] grad The Tvg_Gradient object of which to get the gradient attributes.
 * @param[out] cx The horizontal coordinate of the center of the end circle.
 * @param[out] cy The vertical coordinate of the center of the end circle.
 * @param[out] r The radius of the end circle.
 * @param[out] fx The horizontal coordinate of the center of the start circle.
 * @param[out] fy The vertical coordinate of the center of the start circle.
 * @param[out] fr The radius of the start circle.
 *
 * @see tvg_radial_gradient_set()
 */
TVG_API Tvg_Result tvg_radial_gradient_get(Tvg_Gradient grad, float* cx, float* cy, float* r, float* fx, float* fy, float* fr);

/**
 * @brief Sets the parameters of the colors of the gradient and their position.
 *
 * @param[in] grad The Tvg_Gradient object of which the color information is to be set.
 * @param[in] color_stop An array of Tvg_Color_Stop data structure.
 * @param[in] cnt The size of the @p color_stop array equal to the colors number used in the gradient.
 *
 */
TVG_API Tvg_Result tvg_gradient_set_color_stops(Tvg_Gradient grad, const Tvg_Color_Stop* color_stop, uint32_t cnt);

/**
 * @brief Gets the parameters of the colors of the gradient, their position and number
 *
 * The function does not allocate any memory.
 *
 * @param[in] grad The Tvg_Gradient object of which to get the color information.
 * @param[out] color_stop An array of Tvg_Color_Stop data structure.
 * @param[out] cnt The size of the @p color_stop array equal to the colors number used in the gradient.
 *
 */
TVG_API Tvg_Result tvg_gradient_get_color_stops(const Tvg_Gradient grad, const Tvg_Color_Stop** color_stop, uint32_t* cnt);

/**
 * @brief Sets the Tvg_Stroke_Fill value, which specifies how to fill the area outside the gradient bounds.
 *
 * @param[in] grad The Tvg_Gradient object.
 * @param[in] spread The FillSpread value.
 *
 */
TVG_API Tvg_Result tvg_gradient_set_spread(Tvg_Gradient grad, const Tvg_Stroke_Fill spread);

/**
 * @brief Gets the FillSpread value of the gradient object.
 *
 * @param[in] grad The Tvg_Gradient object.
 * @param[out] spread The FillSpread value.
 *
 */
TVG_API Tvg_Result tvg_gradient_get_spread(const Tvg_Gradient grad, Tvg_Stroke_Fill* spread);

/**
 * @brief Sets the matrix of the affine transformation for the gradient object.
 *
 * The augmented matrix of the transformation is expected to be given.
 *
 * @param[in] grad The Tvg_Gradient object to be transformed.
 * @param[in] m The 3x3 augmented matrix.
 *
 */
TVG_API Tvg_Result tvg_gradient_set_transform(Tvg_Gradient grad, const Tvg_Matrix* m);

/**
 * @brief Gets the matrix of the affine transformation of the gradient object.
 *
 * In case no transformation was applied, the identity matrix is set.
 *
 * @param[in] grad The Tvg_Gradient object of which to get the transformation matrix.
 * @param[out] m The 3x3 augmented matrix.
 *
 */
TVG_API Tvg_Result tvg_gradient_get_transform(const Tvg_Gradient grad, Tvg_Matrix* m);

/**
 * @brief Gets the unique value of the gradient instance indicating the instance type.
 *
 * @param[in] grad The Tvg_Gradient object of which to get the type value.
 * @param[out] type The unique type of the gradient instance type.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_gradient_get_type(const Tvg_Gradient grad, Tvg_Type* type);

/**
 * @brief Duplicates the given Tvg_Gradient object.
 *
 * Creates a new object and sets its all properties as in the original object.
 *
 * @param[in] grad The Tvg_Gradient object to be copied.
 *
 * @return A copied Tvg_Gradient object if succeed, @c nullptr otherwise.
 */
TVG_API Tvg_Gradient tvg_gradient_duplicate(Tvg_Gradient grad);

/**
 * @brief Deletes the given gradient object.
 *
 * @param[in] grad The gradient object to be deleted.
 *
 */
TVG_API Tvg_Result tvg_gradient_del(Tvg_Gradient grad);

/** \} */   // end defgroup ThorVGCapi_Gradient

/**
 * @defgroup ThorVGCapi_Picture Picture
 *
 * @brief A module enabling to create and to load an image in one of the supported formats: svg, png, jpg, lottie and raw.
 *
 *
 * \{
 */

/************************************************************************/
/* Picture API                                                          */
/************************************************************************/

/**
 * @brief Creates a new Picture object.
 *
 * This function allocates and returns a new Picture instance.
 * To properly destroy the Picture object, use @ref tvg_paint_rel().
 *
 * @return The newly created Picture object.
 *
 * @see tvg_paint_rel()
 */
TVG_API Tvg_Paint tvg_picture_new(void);

/**
 * @brief Loads a picture data directly from a file.
 *
 * ThorVG efficiently caches the loaded data using the specified @p path as a key.
 * This means that loading the same file again will not result in duplicate operations;
 * instead, ThorVG will reuse the previously loaded picture data.
 *
 * @param[in] picture The picture object.
 * @param[in] path The absolute path to the image file.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT An empty @p path.
 * @retval TVG_RESULT_NOT_SUPPORTED A file with an unknown extension.
 */
TVG_API Tvg_Result tvg_picture_load(Tvg_Paint picture, const char* path);

/**
 * @brief Loads raw image data in a specific format from a memory block of the given size.
 *
 * ThorVG efficiently caches the loaded data, using the provided @p data address as a key
 * when @p copy is set to @c false. This allows ThorVG to avoid redundant operations
 * by reusing the previously loaded picture data for the same sharable @p data,
 * rather than duplicating the load process.
 *
 * @param[in] picture The picture object.
 * @param[in] data The memory block where the raw image data is stored.
 * @param[in] w The width of the image in pixels.
 * @param[in] h The height of the image in pixels.
 * @param[in] cs Specifies how the 32-bit color values should be interpreted (read/write).
 * @param[in] copy If @c true, the data is copied into the engine's local buffer. If @c false, the data is not copied.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT No data are provided or the @p w or @p h value is zero or less.
 *
 * @since 0.9
 */
TVG_API Tvg_Result tvg_picture_load_raw(Tvg_Paint picture, const uint32_t *data, uint32_t w, uint32_t h, Tvg_Colorspace cs, bool copy);

/**
 * @brief Loads a picture data from a memory block of a given size.
 *
 * ThorVG efficiently caches the loaded data using the specified @p data address as a key
 * when the @p copy has @c false. This means that loading the same data again will not result in duplicate operations
 * for the sharable @p data. Instead, ThorVG will reuse the previously loaded picture data.
 *
 * @param[in] picture The picture object.
 * @param[in] data A pointer to a memory location where the content of the picture file is stored. A null-terminated string is expected for non-binary data if @p copy is @c false
 * @param[in] size The size in bytes of the memory occupied by the @p data.
 * @param[in] mimetype Mimetype or extension of data such as "jpg", "jpeg", "svg", "svg+xml", "lot", "lottie+json", "png", etc. In case an empty string or an unknown type is provided, the loaders will be tried one by one.
 * @param[in] rpath A resource directory path, if the @p data needs to access any external resources.
 * @param[in] copy If @c true the data are copied into the engine local buffer, otherwise they are not.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT In case a @c nullptr is passed as the argument or the @p size is zero or less.
 * @retval TVG_RESULT_NOT_SUPPORTED A file with an unknown extension.
 *
 * @warning: It's the user responsibility to release the @p data memory if the @p copy is @c true.
 */
TVG_API Tvg_Result tvg_picture_load_data(Tvg_Paint picture, const char *data, uint32_t size, const char *mimetype, const char* rpath, bool copy);

/**
 * @brief Sets the asset resolver callback for handling external resources (e.g., images and fonts).
 *
 * This callback is invoked when an external asset reference (such as an image source or file path)
 * is encountered in a Picture object. It allows the user to provide a custom mechanism for loading
 * or substituting assets, such as loading from an external source or a virtual filesystem.
 *
 * @param[in] resolver A user-defined function that handles the resolution of asset paths.
 *                     The function should return @c true if the asset was successfully resolved by the user, or @c false if it was not.
 * @param[in] data A pointer to user-defined data that will be passed to the callback each time it is invoked.
 *                 This can be used to maintain context or access external resources.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT A @c nullptr passed as the @p picture argument.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the @p picture is already loaded.
 *
 * @note This function must be called before @ref tvg_picture_load()
 *       Setting the resolver after loading will have no effect on asset resolution for that asset.
 * @note If @c false is returned by @p resolver, ThorVG will attempt to resolve the resource using its internal resolution mechanism as a fallback.
 * @note To unset the resolver, pass @c nullptr as the @p resolver parameter.
 * @note Experimental API
 *
 * @see Tvg_Picture_Asset_Resolver
 */
TVG_API Tvg_Result tvg_picture_set_asset_resolver(Tvg_Paint picture, Tvg_Picture_Asset_Resolver resolver, void* data);

/**
 * @brief Resizes the picture content to the given width and height.
 *
 * The picture content is resized while keeping the default size aspect ratio.
 * The scaling factor is established for each of dimensions and the smaller value is applied to both of them.
 *
 * @param[in] picture The picture object.
 * @param[in] w A new width of the image in pixels.
 * @param[in] h A new height of the image in pixels.
 *
 */
TVG_API Tvg_Result tvg_picture_set_size(Tvg_Paint picture, float w, float h);

/**
 * @brief Gets the size of the loaded picture.
 *
 * @param[in] picture The picture object.
 * @param[out] w A width of the image in pixels.
 * @param[out] h A height of the image in pixels.
 *
 */
TVG_API Tvg_Result tvg_picture_get_size(const Tvg_Paint picture, float* w, float* h);

/**
 * @brief Sets the normalized origin point of the Picture object.
 *
 * This method defines the origin point of the Picture using normalized coordinates.
 * Unlike a typical pivot point used only for transformations, this origin affects both
 * the transformation behavior and the actual rendering position of the Picture.
 *
 * The specified origin becomes the reference point for positioning the Picture on the canvas.
 * For example, setting the origin to (0.5f, 0.5f) moves the visual center of the picture
 * to the position specified by Paint::translate().
 *
 * The coordinates are given in a normalized range relative to the picture's bounds:
 * - (0.0f, 0.0f): top-left corner
 * - (0.5f, 0.5f): center
 * - (1.0f, 1.0f): bottom-right corner
 *
 * @param[in] picture The picture object.
 * @param[in] x The normalized x-coordinate of the origin point (range: 0.0f to 1.0f).
 * @param[in] y The normalized y-coordinate of the origin point (range: 0.0f to 1.0f).
 *
 * @note This origin directly affects how the Picture is placed on the canvas when using
 *       transformations such as translate(), rotate(), or scale().
 *
 * @see tvg_paint_translate()
 * @see tvg_paint_rotate()
 * @see tvg_paint_scale()
 * @see tvg_paint_set_transform()
 * @see tvg_picture_get_origin()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_picture_set_origin(Tvg_Paint picture, float x, float y);

/**
 * @brief Gets the normalized origin point of the Picture object.
 *
 * This method retrieves the current origin point of the Picture, expressed
 * in normalized coordinates relative to the picture’s bounds.
 *
 * @param[in] picture The picture object.
 * @param[out] x The normalized x-coordinate of the origin (range: 0.0f to 1.0f).
 * @param[out] y The normalized y-coordinate of the origin (range: 0.0f to 1.0f).
 *
 * @see tvg_picture_set_origin()
 * @since 1.0
 */
TVG_API Tvg_Result tvg_picture_get_origin(const Tvg_Paint picture, float* x, float* y);

/**
 * @brief Retrieve a paint object from the Picture scene by its Unique ID.
 *
 * This function searches for a paint object within the Picture scene that matches the provided @p id.
 *
 * @param[in] picture The picture object.
 * @param[in] id The Unique ID of the paint object.
 *
 * @return The paint object that matches the given identifier, or @c nullptr if no matching paint object is found.
 *
 * @note Setting @ref tvg_picture_set_accessible() to @c true enables more efficient access.
 *
 * @see tvg_accessor_generate_id()
 * @since 1.0
 */
TVG_API const Tvg_Paint tvg_picture_get_paint(Tvg_Paint picture, uint32_t id);

/**
 * @brief Sets the image filtering method for rendering this picture.
 *
 * Specifies how the image data should be filtered when it is scaled or transformed
 * during rendering. This affects the visual quality and performance of the output.
 *
 * @param[in] picture A Tvg_Paint pointer to the picture object.
 * @param[in] method The filtering method to apply. Default is @c TVG_FILTER_METHOD_BILINEAR.
 *
 * @see Tvg_Filter_Method
 * @since 1.1
 */
TVG_API Tvg_Result tvg_picture_set_filter(Tvg_Paint picture, Tvg_Filter_Method method);

/**
 * @brief Enable or disable accessible mode for a Picture.
 *
 * When accessible mode is enabled, the Picture maintains an internal mapping
 * of ID-accessible vector assets nodes (such as SVG), allowing efficient access to Paint objects
 * and their associated identifier information via Accessor APIs.
 *
 * When disabled, no additional mapping is maintained and all nodes are treated
 * as general traversal targets.
 *
 * @param[in] picture The target Picture object.
 * @param[in] accessible Set to @c true to enable accessible mode, or @c false to disable it.
 *
 * @see tvg_accessor_generate_id()
 * @see tvg_accessor_get_name()
 * @see tvg_picture_get_paint()
 *
 * @since 1.1
 */
TVG_API Tvg_Result tvg_picture_set_accessible(Tvg_Paint picture, bool accessible);

/** \} */   // end defgroup ThorVGCapi_Picture

/**
 * @defgroup ThorVGCapi_Scene Scene
 * @brief A module managing the multiple paints as one group paint.
 *
 * As a group, scene can be transformed, translucent, composited with other target paints,
 * its children will be affected by the scene world.
 *
 * \{
 */

/************************************************************************/
/* Scene API                                                            */
/************************************************************************/

/**
 * @brief Creates a new Scene object.
 *
 * This function allocates and returns a new Scene instance.
 * To properly destroy the Scene object, use @ref tvg_paint_rel().
 *
 * @return The newly created Scene object.
 *
 * @see tvg_paint_rel()
 */
TVG_API Tvg_Paint tvg_scene_new(void);

/**
 * @brief Adds a paint object to the scene.
 *
 * Appends the specified paint object to the given scene. Only paint objects
 * added to the scene are considered rendering targets.
 *
 * @param[in] scene A handle to the scene object.
 *                  This parameter must not be @c nullptr.
 * @param[in] paint A handle to the paint object to be added to the scene.
 *                  This parameter must not be @c nullptr.
 *
 * @note Ownership of the @p paint object is transferred to the canvas upon
 *       successful addition. To retain ownership, call @ref tvg_paint_ref()
 *       before adding it to the scene.
* @note The rendering order of paint objects follows their order in the root
 *       scene. If layering is required, ensure the paints are added in the
 *       desired order.
 *
 * @see tvg_scene_insert()
 * @see tvg_scene_remove()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add(Tvg_Paint scene, Tvg_Paint paint);

/**
 * @brief Inserts a paint object into the scene.
 *
 * Inserts the specified paint object into the scene immediately before the
 * given paint object @p at. The @p at parameter must reference an existing
 * paint object already added to the scene.
 *
 * @param[in] scene  A handle to the scene object.
 *                   This parameter must not be @c nullptr.
 * @param[in] target A handle to the paint object to be inserted into the scene.
 *                   This parameter must not be @c nullptr.
 * @param[in] at     A handle to an existing paint object in the scene before
 *                   which @p target will be inserted.
 *                   This parameter must not be @c nullptr.
 *
 * @note Ownership of the @p target object is transferred to the scene upon
 *       successful addition. To retain ownership, call @ref tvg_paint_ref()
 *       before adding it to the scene.
 * @note The rendering order of paint objects follows their order in the root
 *       scene. If layering is required, ensure the paints are added in the
 *       desired order.
 *
 * @see tvg_scene_add()
 * @see tvg_scene_remove()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_insert(Tvg_Paint scene, Tvg_Paint target, Tvg_Paint at);

/**
 * @brief Removes a paint object from the scene.
 *
 * This function removes a specified paint object from the scene. If no paint
 * object is specified (i.e., the default @c nullptr is used), the function
 * performs to clear all paints from the scene.
 *
 * @param[in] scene The scene object.
 * @param[in] paint The paint object to be removed from the scene.
 *                  If @c nullptr, remove all the paints from the scene.
 *
 * @see tvg_scene_add()
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_remove(Tvg_Paint scene, Tvg_Paint paint);

/**
 * @brief Clears all previously applied scene effects.
 *
 * This function clears all effects that have been applied to the scene,
 * restoring it to its original state without any post-processing.
 *
 * @param[in] scene The scene object.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_clear_effects(Tvg_Paint scene);

/**
 * @brief Adds a Gaussian blur effect to the scene.
 *
 * This function adds a Gaussian blur filter to the scene as a post-processing effect.
 * The blur can be applied in different directions with configurable border handling and quality settings.
 *
 * @param[in] scene The scene object.
 * @param[in] sigma The blur radius (sigma) value. Must be greater than 0.
 * @param[in] direction Blur direction: 0 = both directions, 1 = horizontal only, 2 = vertical only.
 * @param[in] border Border handling method: 0 = duplicate, 1 = wrap.
 * @param[in] quality Blur quality level [0 - 100].
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add_effect_gaussian_blur(Tvg_Paint scene, double sigma, int direction, int border, int quality);

/**
 * @brief Adds a drop shadow effect to the scene.
 *
 * This function adds a drop shadow with a Gaussian blur to the scene. The shadow 
 * can be customized using color, opacity, angle, distance, blur radius (sigma), 
 * and quality parameters.
 *
 * @param[in] scene The scene object.
 * @param[in] r Red channel value of the shadow color [0 - 255].
 * @param[in] g Green channel value of the shadow color [0 - 255].
 * @param[in] b Blue channel value of the shadow color [0 - 255].
 * @param[in] a Alpha (opacity) channel value of the shadow [0 - 255].
 * @param[in] angle Shadow direction in degrees [0 - 360].
 * @param[in] distance Distance of the shadow from the original object.
 * @param[in] sigma Gaussian blur sigma value for the shadow. Must be > 0.
 * @param[in] quality Blur quality level [0 - 100].
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add_effect_drop_shadow(Tvg_Paint scene, int r, int g, int b, int a, double angle, double distance, double sigma, int quality);

/**
 * @brief Adds a fill color effect to the scene.
 *
 * This function overrides the scene's content colors with the specified fill color.
 *
 * @param[in] scene The scene object.
 * @param[in] r Red color channel value [0 - 255].
 * @param[in] g Green color channel value [0 - 255].
 * @param[in] b Blue color channel value [0 - 255].
 * @param[in] a Alpha (opacity) channel value [0 - 255].
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add_effect_fill(Tvg_Paint scene, int r, int g, int b, int a);

/**
 * @brief Adds a tint effect to the scene.
 *
 * This function tints the current scene using specified black and white color values,
 * modulated by a given intensity.
 *
 * @param[in] scene The scene object.
 * @param[in] black_r Red component of the black color [0 - 255].
 * @param[in] black_g Green component of the black color [0 - 255].
 * @param[in] black_b Blue component of the black color [0 - 255].
 * @param[in] white_r Red component of the white color [0 - 255].
 * @param[in] white_g Green component of the white color [0 - 255].
 * @param[in] white_b Blue component of the white color [0 - 255].
 * @param[in] intensity Tint intensity value [0 - 100].
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add_effect_tint(Tvg_Paint scene, int black_r, int black_g, int black_b, int white_r, int white_g, int white_b, double intensity);

/**
 * @brief Adds a tritone color effect to the scene.
 *
 * This function adds a tritone color effect to the given scene using three sets of RGB values 
 * representing shadow, midtone, and highlight colors.
 *
 * @param[in] scene The scene object.
 * @param[in] shadow_r Red component of the shadow color [0 - 255].
 * @param[in] shadow_g Green component of the shadow color [0 - 255].
 * @param[in] shadow_b Blue component of the shadow color [0 - 255].
 * @param[in] midtone_r Red component of the midtone color [0 - 255].
 * @param[in] midtone_g Green component of the midtone color [0 - 255].
 * @param[in] midtone_b Blue component of the midtone color [0 - 255].
 * @param[in] highlight_r Red component of the highlight color [0 - 255].
 * @param[in] highlight_g Green component of the highlight color [0 - 255].
 * @param[in] highlight_b Blue component of the highlight color [0 - 255].
 * @param[in] blend A blending factor that determines the mix between the original color and the tritone colors [0 - 255].
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_scene_add_effect_tritone(Tvg_Paint scene, int shadow_r, int shadow_g, int shadow_b, int midtone_r, int midtone_g, int midtone_b, int highlight_r, int highlight_g, int highlight_b, int blend);

/** \} */   // end defgroup ThorVGCapi_Scene

/**
 * @defgroup ThorVGCapi_Text Text
 * @brief A class to represent text objects in a graphical context, allowing for rendering and manipulation of unicode text.
 *
 * @since 0.15
 *
 * \{
 */

/************************************************************************/
/* Text API                                                            */
/************************************************************************/

/**
 * @brief Creates a new Text object.
 *
 * This function allocates and returns a new Text instance.
 * To properly destroy the Text object, use @ref tvg_paint_rel().
 *
 * @return The newly created Text object.
 *
 * @see tvg_paint_rel()
 *
 * @since 0.15
 */
TVG_API Tvg_Paint tvg_text_new(void);

/**
 * @brief Sets the font family for the text.
 *
 * This function specifies the name of the font to be used when rendering text.
 *
 * @param[in] text The text object.
 * @param[in] name The name of the font. This should match a font available through the canvas backend.
 *                 If set to @c nullptr, ThorVG will attempt to select a fallback font available on the engine.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION  The specified @p name cannot be found.
 *
 * @note This function only sets the font family name. Use @ref size() to define the font size.
 * @note If the @p name is not specified, ThorVG will select an available fallback font.
 *
 * @see tvg_text_set_size()
 * @see tvg_font_load()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_set_font(Tvg_Paint text, const char* name);

/**
 * @brief Sets the font size for the text.
 *
 * This function sets the font size used during text rendering.
 * The size is specified in point units, and supports floating-point precision
 * for smooth scaling and animation effects.
 *
 * @param[in] text The text object.
 * @param[in] size The font size in points. Must be greater than 0.0.
 *
  * @retval TVG_RESULT_INVALID_ARGUMENT if the @p size is less than or equal to 0.
 *
 * @note Use this function in combination with @ref font() to fully define text appearance.
 * @note Fractional sizes (e.g., 12.5) are supported for sub-pixel rendering and animations.
 *
 * @see tvg_text_set_font()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_set_size(Tvg_Paint text, float size);

/**
 * @brief Assigns the given unicode text to be rendered.
 *
 * This function sets the unicode text that will be displayed by the rendering system.
 * The text is set according to the specified UTF encoding method, which defaults to UTF-8.
 *
 * @param[in] text The text object.
 * @param[in] utf8 The multi-byte text encoded with utf8 string to be rendered.
 *
 * @see tvg_text_get_text()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_set_text(Tvg_Paint text, const char* utf8);

 /**
  * @brief Returns the currently assigned unicode text.
  *
  * This function retrieves the unicode string that is currently set
  * for rendering. The returned text is encoded in UTF-8.
  *
  * @param[in] text The text object.
  *
  * @return The UTF-8 encoded multi-byte text string.
  *
  * @see tvg_text_set_text()
  *
  * @note Experimental API
  */
 TVG_API const char* tvg_text_get_text(const Tvg_Paint text);

/**
 * @brief Sets text alignment or anchor per axis.
 *
 * If layout width/height is set on an axis, align within the layout box.
 * Otherwise, treat it as an anchor within the text bounds which point of
 * the text box is pinned to the paint position.
 *
 * @param[in] text The text object.
 * @param[in] x Horizontal alignment/anchor in [0..1]: 0=left/start, 0.5=center, 1=right/end. (Default is 0)
 * @param[in] y Vertical alignment/anchor in [0..1]: 0=top, 0.5=middle, 1=bottom. (Default is 0)
 *
 * @since 1.0
 *
 * @see tvg_text_layout()
 */
TVG_API Tvg_Result tvg_text_align(Tvg_Paint text, float x, float y);

/**
 * @brief Sets the virtual layout box (constraints) for the text.
 *
 * If width/height is set on an axis, that axis is constrained by a virtual layout box and
 * the text may wrap/align inside it. If width/height == 0, the axis is
 * unconstrained and @ref tvg_text_align() acts as an anchor on that axis.
 *
 * @param[in] text The text object.
 * @param[in] w Layout width in user space. Use 0 for no horizontal constraint. (Default is 0)
 * @param[in] h Layout height in user space. Use 0 for no vertical constraint. (Default is 0)
 *
 * @note This defines constraints only; alignment/anchoring is controlled by @ref align().
 * @since 1.0
 *
 * @see tvg_text_align()
 * @see tvg_text_spacing()
 */
TVG_API Tvg_Result tvg_text_layout(Tvg_Paint text, float w, float h);

/**
 * @brief Sets the text wrapping mode for this text object.
 *
 * This method controls how the text is laid out when it exceeds the available space.
 * The wrapping mode determines whether text is truncated, wrapped by character or word,
 * or adjusted automatically. An ellipsis mode is also available for truncation with "...".
 *
 * @param[in] text The text object.
 * @param[in] mode The wrapping strategy to apply. Default is @c TVG_TEXT_WRAP_NONE.
 *
 * @see Tvg_Text_Wrap
 * @see tvg_text_line_count()
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_wrap_mode(Tvg_Paint text, Tvg_Text_Wrap mode);

/**
 * @brief Returns the number of text lines.
 *
 * This function retrieves the number of lines generated after applying text layout and wrapping.
 * The returned value reflects the current wrapping configuration set by tvg_text_wrap_mode().
 * The line count is also increased by explicit line feed characters ('\n') contained in the text.
 *
 * @param[in] text The text object.
 *
 * @return The total number of lines.
 *
 * @see tvg_text_wrap_mode()
 * @since 1.1
 */
TVG_API uint32_t tvg_text_line_count(Tvg_Paint text);

/**
 * @brief Set the spacing scale factors for text layout.
 *
 * This function adjusts the letter spacing (horizontal space between glyphs) and
 * line spacing (vertical space between lines of text) using scale factors.
 *
 * Both values are relative to the font's default metrics:
 * - The letter spacing is applied as a scale factor to the glyph's advance width.
 * - The line spacing is applied as a scale factor to the glyph's advance height.
 *
 * @param[in] text The text object.
 * @param[in] letter The scale factor for letter spacing.
 *                   Values > 1.0 increase spacing, values < 1.0 decrease it.
 *                   Must be greater than or equal to 0.0. (default: 1.0)
 *
 * @param[in] line The scale factor for line spacing.
 *                 Values > 1.0 increase line spacing, values < 1.0 decrease it.
 *                 Must be greater than or equal to 0.0. (default: 1.0)
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_spacing(Tvg_Paint text, float letter, float line);

/**
 * @brief Apply an italic (slant) transformation to the text.
 *
 * This function applies a shear transformation to simulate an italic (oblique) style
 * for the current text object. The shear factor determines the degree of slant
 * applied along the X-axis.
 *
 * @param[in] text The text object.
 * @param[in] shear The shear factor to apply. A value of 0.0 applies no slant, while values around 0.5 result in a strong slant.
 *                  Must be in the range [0.0, 0.5]. Recommended value is 0.18.
 *
 * @note The @p shear factor will be clamped to the valid range if it exceeds the limits.
 * @note This does not require the font itself to be italic.
 *       It visually simulates the effect by applying a transformation matrix.
 *
 * @warning Excessive slanting may cause visual distortion depending on the font and size.
 *
 * @see tvg_text_set_font()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_set_italic(Tvg_Paint text, float shear);

/**
 * @brief Sets an outline (stroke) around the text object.
 *
 * This function adds an outline to the text with the specified width and RGB color.
 * The outline enhances the visibility of the text by rendering a stroke around its glyphs.
 *
 * @param[in] text The text object.
 * @param width The width of the outline. Must be positive value. (The default is 0)
 * @param r     Red component of the outline color (0–255).
 * @param g     Green component of the outline color (0–255).
 * @param b     Blue component of the outline color (0–255).
 *
 * @note To disable the outline, set @p width to 0.
 * @see tvg_text_set_fill_color() to set the main text fill color.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_text_set_outline(Tvg_Paint text, float width, uint8_t r, uint8_t g, uint8_t b);

/**
 * @brief Sets the text solid color.
 *
 * @param[in] paint The text object.
 * @param[in] r The red color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] g The green color channel value in the range [0 ~ 255]. The default value is 0.
 * @param[in] b The blue color channel value in the range [0 ~ 255]. The default value is 0.
 *
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 *
 * @see tvg_text_set_font()
 * @see tvg_text_set_outline()
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_text_set_color(Tvg_Paint text, uint8_t r, uint8_t g, uint8_t b);

/**
 * @brief Sets the gradient fill for the text.
 *
 * @param[in] text The text object.
 * @param[in] grad The linear or radial gradient fill
 *
 * @retval TVG_RESULT_MEMORY_CORRUPTION An invalid gradient object.
 *
 * @note Either a solid color or a gradient fill is applied, depending on what was set as last.
 * @see tvg_text_set_font()
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_text_set_gradient(Tvg_Paint text, Tvg_Gradient gradient);

/**
 * @brief Retrieves the layout metrics of the text object.
 *
 * Fills the provided @ref Tvg_Text_Metrics structure with the font layout values of this text object,
 * such as ascent, descent, linegap, and line advance.
 *
 * The returned values reflect the font size applied to the text object,
 * but do not include any transformations (e.g., scale, rotation, or translation).
 *
 * @param[in] text The text object.
 * @param[out] metrics A pointer to a @ref Tvg_Text_Metrics structure to be filled with the resulting values.
 *
 * @return TVG_RESULT_INSUFFICIENT_CONDITION if no font or size has been set yet.
 *
 * @see Tvg_Text_Metrics
 * @note Experimental API
 */
TVG_API Tvg_Result tvg_text_get_text_metrics(const Tvg_Paint text, Tvg_Text_Metrics* metrics);

/**
 * @brief Retrieves the layout metrics of a glyph in the text object.
 *
 * Fills the provided @ref Tvg_Glyph_Metrics structure with the horizontal layout values
 * of the specified glyph, such as advance, left-side bearing, and bounding box.
 *
 * The returned values reflect the font size applied to the text object,
 * but do not include any transformations (e.g., scale, rotation, or translation).
 *
 * The input character must be a single UTF-8 encoded character.
 *
 * @param[in] text The text object.
 * @param[in] ch A pointer to a UTF-8 encoded character.
 * @param[out] metrics A pointer to a @ref Tvg_Glyph_Metrics structure to be filled with the resulting values.
 * @param[out] next An optional pointer that receives the position immediately
 *                  following the processed UTF-8 character.
 *
 * @return TVG_RESULT_INSUFFICIENT_CONDITION if no font or size has been set yet.
 * @return TVG_RESULT_INVALID_ARGUMENT if the given character is invalid or not supported.
 *
 * @see Tvg_Glyph_Metrics
 * @note Currently, ThorVG only supports horizontal text layout.
 * @note Experimental API
 */
TVG_API Tvg_Result tvg_text_get_glyph_metrics(const Tvg_Paint text, const char* ch, Tvg_Glyph_Metrics* metrics, const char** next);

/**
 * @brief Loads a scalable font data from a file.
 *
 * ThorVG efficiently caches the loaded data using the specified @p path as a key.
 * This means that loading the same file again will not result in duplicate operations;
 * instead, ThorVG will reuse the previously loaded font data.
 *
 * @param[in] path The path to the font file.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT An invalid @p path passed as an argument.
 * @retval TVG_RESULT_NOT_SUPPORTED When trying to load a file with an unknown extension.
 *
 * @see tvg_font_unload()
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_font_load(const char* path);

/**
 * @brief Loads a scalable font data from a memory block of a given size.
 *
 * ThorVG efficiently caches the loaded font data using the specified @p name as a key.
 * This means that loading the same fonts again will not result in duplicate operations.
 * Instead, ThorVG will reuse the previously loaded font data.
 *
 * @param[in] name The name under which the font will be stored and accessible (e.x. in a @p tvg_text_set_font API).
 * @param[in] data A pointer to a memory location where the content of the font data is stored.
 * @param[in] size The size in bytes of the memory occupied by the @p data.
 * @param[in] mimetype Mimetype or extension of font data. In case a @c nullptr or an empty "" value is provided the loader will be determined automatically.
 * @param[in] copy If @c true the data are copied into the engine local buffer, otherwise they are not (default).
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT If no name is provided or if @p size is zero while @p data points to a valid memory location.
 * @retval TVG_RESULT_NOT_SUPPORTED When trying to load a file with an unknown extension.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION When trying to unload the font data that has not been previously loaded.
 *
 * @warning: It's the user responsibility to release the @p data memory.
 *
 * @note To unload the font data loaded using this API, pass the proper @p name and @c nullptr as @p data.
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_font_load_data(const char* name, const char* data, uint32_t size, const char *mimetype, bool copy);

/**
 * @brief Unloads the specified scalable font data that was previously loaded.
 *
 * This function is used to release resources associated with a font file that has been loaded into memory.
 *
 * @param[in] path The path to the loaded font file.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION The loader is not initialized.
 *
 * @note If the font data is currently in use, it will not be immediately unloaded.
 * @see tvg_font_load()
 *
 * @since 0.15
 */
TVG_API Tvg_Result tvg_font_unload(const char* path);

/** \} */   // end defgroup ThorVGCapi_Text

/**
 * @defgroup ThorVGCapi_Saver Saver
 * @brief A module for exporting a paint object into a specified file.
 *
 * The module enables to save the composed scene and/or image from a paint object.
 * Once it's successfully exported to a file, it can be recreated using the Picture module.
 *
 * \{
 */

/************************************************************************/
/* Saver API                                                            */
/************************************************************************/
/**
 * @brief Creates a new Tvg_Saver object.
 *
 * @return A new Tvg_Saver object.
 */
TVG_API Tvg_Saver tvg_saver_new(void);

/**
 * @brief Exports the given @p paint data to the given @p path
 *
 * If the saver module supports any compression mechanism, it will optimize the data size.
 * This might affect the encoding/decoding time in some cases. You can turn off the compression
 * if you wish to optimize for speed.
 *
 * @param[in] saver The Tvg_Saver object connected with the saving task.
 * @param[in] paint The paint to be saved with all its associated properties.
 * @param[in] path A path to the file, in which the paint data is to be saved.
 * @param[in] quality The encoded quality level. @c 0 is the minimum, @c 100 is the maximum value(recommended).
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION Currently saving other resources.
 * @retval TVG_RESULT_NOT_SUPPORTED Trying to save a file with an unknown extension or in an unsupported format.
 * @retval TVG_RESULT_UNKNOWN An empty paint is to be saved.
 *
 * @note Saving can be asynchronous if the assigned thread number is greater than zero. To guarantee the saving is done, call tvg_saver_sync() afterwards.
 * @see tvg_saver_sync()
 */
TVG_API Tvg_Result tvg_saver_save_paint(Tvg_Saver saver, Tvg_Paint paint, const char* path, uint32_t quality);

/**
 * @brief Exports the given @p animation data to the given @p path
 *
 * If the saver module supports any compression mechanism, it will optimize the data size.
 * This might affect the encoding/decoding time in some cases. You can turn off the compression
 * if you wish to optimize for speed.
 *
 * @param[in] saver The Tvg_Saver object connected with the saving task.
 * @param[in] animation The animation to be saved with all its associated properties.
 * @param[in] path A path to the file, in which the animation data is to be saved.
 * @param[in] quality The encoded quality level. @c 0 is the minimum, @c 100 is the maximum value(recommended).
 * @param[in] fps The frames per second for the animation. If @c 0, the default fps is used.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION Currently saving other resources or animation has no frames.
 * @retval TVG_RESULT_NOT_SUPPORTED Trying to save a file with an unknown extension or in an unsupported format.
 * @retval TVG_RESULT_UNKNOWN Unknown if attempting to save an empty paint.
 *
 * @note A higher frames per second (FPS) would result in a larger file size. It is recommended to use the default value.
 * @note Saving can be asynchronous if the assigned thread number is greater than zero. To guarantee the saving is done, call tvg_saver_sync() afterwards.
 *
 * @see tvg_saver_sync()
 *
 * @since 1.0
*/
TVG_API Tvg_Result tvg_saver_save_animation(Tvg_Saver saver, Tvg_Animation animation, const char* path, uint32_t quality, uint32_t fps);

/**
 * @brief Guarantees that the saving task is finished.
 *
 * The behavior of the Saver module works on a sync/async basis, depending on the threading setting of the Initializer.
 * Thus, if you wish to have a benefit of it, you must call tvg_saver_sync() after the tvg_saver_save_paint() in the proper delayed time.
 * Otherwise, you can call tvg_saver_sync() immediately.
 *
 * @param[in] saver The Tvg_Saver object connected with the saving task.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION No saving task is running.
 *
 * @note The asynchronous tasking is dependent on the Saver module implementation.
 * @see tvg_saver_save_paint()
 */
TVG_API Tvg_Result tvg_saver_sync(Tvg_Saver saver);

/**
 * @brief Deletes the given Tvg_Saver object.
 *
 * @param[in] saver The Tvg_Saver object to be deleted.
 *
 */
TVG_API Tvg_Result tvg_saver_del(Tvg_Saver saver);

/** \} */   // end defgroup ThorVGCapi_Saver

/**
 * @defgroup ThorVGCapi_Animation Animation
 * @brief A module for manipulation of animatable images.
 *
 * The module supports the display and control of animation frames.
 *
 * \{
 */

/************************************************************************/
/* Animation API                                                        */
/************************************************************************/

/**
 * @brief Creates a new Animation object.
 *
 * @return Tvg_Animation A new Tvg_Animation object.
 *
 * @since 0.13
 */
TVG_API Tvg_Animation tvg_animation_new(void);

/**
 * @brief Specifies the current frame in the animation.
 *
 * @param[in] animation An animation object.
 * @param[in] no The index of the animation frame to be displayed. The index should be less than the tvg_animation_get_total_frame().
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION if the given @p no is the same as the current frame value.
 * @retval TVG_RESULT_NOT_SUPPORTED The picture data does not support animations.
 *
 * @note For efficiency, ThorVG ignores updates to the new frame value if the difference from the current frame value
 *       is less than 0.001. In such cases, it returns @c Result::InsufficientCondition.
 *       Values less than 0.001 may be disregarded and may not be accurately retained by the Animation.
 * @see tvg_animation_get_total_frame()
 *
 * @since 0.13
*/
TVG_API Tvg_Result tvg_animation_set_frame(Tvg_Animation animation, float no);

/**
 * @brief Retrieves a picture instance associated with this animation instance.
 *
 * This function provides access to the picture instance that can be used to load animation formats, such as lot.
 * After setting up the picture, it can be added to the designated canvas, enabling control over animation frames
 * with this Animation instance.
 *
 * @param[in] animation A animation object to the animation object.
 *
 * @return A picture instance that is tied to this animation.
 *
 * @warning The picture instance is owned by Animation. It should not be deleted manually.
 *
 * @since 0.13
 */
TVG_API Tvg_Paint tvg_animation_get_picture(Tvg_Animation animation);

/**
 * @brief Retrieves the current frame number of the animation.
 *
 * @param[in] animation An animation object.
 * @param[in] no The current frame number of the animation, between 0 and totalFrame() - 1.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT An invalid @p no pointer.
 *
 * @see tvg_animation_get_total_frame()
 * @see tvg_animation_set_frame()
 *
 * @since 0.13
 */
TVG_API Tvg_Result tvg_animation_get_frame(Tvg_Animation animation, float* no);

/**
 * @brief Retrieves the total number of frames in the animation.
 *
 * @param[in] animation An animation object.
 * @param[in] cnt The total number of frames in the animation.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT An invalid @p cnt pointer.
 *
 * @note Frame numbering starts from 0.
 * @note If the Picture is not properly configured, this function will return 0.
 *
 * @since 0.13
 */
TVG_API Tvg_Result tvg_animation_get_total_frame(Tvg_Animation animation, float* cnt);

/**
 * @brief Retrieves the duration of the animation in seconds.
 *
 * @param[in] animation An animation object.
 * @param[in] duration The duration of the animation in seconds.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT An invalid @p duration pointer.
 *
 * @note If the Picture is not properly configured, this function will return 0.
 *
 * @since 0.13
 */
TVG_API Tvg_Result tvg_animation_get_duration(Tvg_Animation animation, float* duration);

/**
 * @brief Specifies the playback segment of the animation.
 *
 * The set segment is designated as the play area of the animation.
 * This is useful for playing a specific segment within the entire animation.
 * After setting, the number of animation frames and the playback time are calculated
 * by mapping the playback segment as the entire range.
 *
 * @param[in] animation The animation object.
 * @param[in] begin segment begin frame.
 * @param[in] end segment end frame.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 * @retval TVG_RESULT_INVALID_ARGUMENT If the @p begin is higher than @p end.
 *
 * @note Animation allows a range from 0.0 to the total frame. @p end should not be lower than @p begin.
 * @note If a marker has been specified, its range will be disregarded.
 *
 * @see tvg_lottie_animation_set_marker()
 * @see tvg_animation_get_total_frame()
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_animation_set_segment(Tvg_Animation animation, float begin, float end);

/**
 * @brief Gets the current segment range information.
 *
 * @param[in] animation The animation object.
 * @param[out] begin segment begin frame.
 * @param[out] end segment end frame.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_animation_get_segment(Tvg_Animation animation, float* begin, float* end);

/**
 * @brief Deletes the given Tvg_Animation object.
 *
 * @param[in] animation The Tvg_Animation object to be deleted.
 *
 * @since 0.13
 */
TVG_API Tvg_Result tvg_animation_del(Tvg_Animation animation);

/** \} */   // end defgroup ThorVGCapi_Animation

/**
 * @defgroup ThorVGCapi_Accesssor Accessor
 * @brief A module for manipulation of the scene tree
 *
 * This module helps to control the scene tree.
 * \{
 */

/************************************************************************/
/* Accessor API                                                         */
/************************************************************************/
/**
 * @brief Creates a new accessor object.
 *
 * @return A new accessor object.
 *
 * @since 1.0
 */
TVG_API Tvg_Accessor tvg_accessor_new(void);

/**
 * @brief Deletes the given accessor object.
 *
 * @param[in] accessor The accessor object to be deleted.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_accessor_del(Tvg_Accessor accessor);

/**
 * @brief Sets the paint of the accessor then iterates through its descendents.
 *
 * Iterates through all descendents of the scene passed through the paint argument
 * while calling func on each and passing the data pointer to this function. When
 * func returns false iteration stops and the function returns.
 *
 * @param[in] accessor An accessor object.
 * @param[in] paint The scene object.
 * @param[in] func A function pointer to the function that will be execute for each child.
 * @param[in] data A void pointer to data that will be passed to the func.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_accessor_set(Tvg_Accessor accessor, Tvg_Paint paint, bool (*func)(Tvg_Paint paint, void* data), void* data);

/**
 * @brief Generate a unique ID (hash key) from a given name.
 *
 * This function computes a unique identifier value based on the provided string.
 * You can use this to assign a unique ID to the Paint object.
 *
 * @param[in] name The input string to generate the unique identifier from.
 *
 * @return The generated unique identifier value.
 *
 * @since 1.0
 */
TVG_API uint32_t tvg_accessor_generate_id(const char* name);

/**
 * @brief Retrieve the original name string from a given unique ID.
 *
 * Returns the name associated with the specified identifier.
 *
 * This method is only valid when @ref tvg_picture_set_accessible() is set to @c true
 * for the Picture associated with the given @p paint in @ref tvg_accessor_set() Otherwise, the name
 * information may not be available.
 *
 * @param[in] accessor An accessor object.
 * @param[in] id The unique identifier.
 *
 * @return The corresponding name string, or @c nullptr if not found or unavailable.
 *
 * @see tvg_accessor_generate_id()
 * @see tvg_accessor_set()
 * @see tvg_picture_set_accessible()
 *
 * @note This function is only available within Accessor callbacks registered via @ref tvg_accessor_set().
 * @since 1.1
 */
TVG_API const char* tvg_accessor_get_name(Tvg_Accessor accessor, uint32_t id);

/** \} */   // end defgroup ThorVGCapi_Accessor

/**
 * @defgroup ThorVGCapi_LottieAnimation LottieAnimation
 * @brief A module for manipulation of lottie extension features.
 *
 * The module enables control of advanced Lottie features.
 * \{
 */

/************************************************************************/
/* LottieAnimation Extension API                                        */
/************************************************************************/

/**
 * @brief Creates a new LottieAnimation object.
 *
 * @return Tvg_Animation A new Tvg_LottieAnimation object.
 *
 * @since 0.15
 */
TVG_API Tvg_Animation tvg_lottie_animation_new(void);

/**
 * @brief Generates a new slot from the given slot data.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] slot The Lottie slot data in JSON format.
 *
 * @return The generated slot ID when successful, 0 otherwise.
 *
 * @since 1.0
 */
TVG_API uint32_t tvg_lottie_animation_gen_slot(Tvg_Animation animation, const char* slot);

/**
 * @brief Applies a previously generated slot to the animation.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] id The ID of the slot to apply, or 0 to reset all slots.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 * @retval TVG_RESULT_INVALID_ARGUMENT When the given @p id is invalid
 * @retval TVG_RESULT_NOT_SUPPORTED The Lottie Animation is not supported.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_apply_slot(Tvg_Animation animation, uint32_t id);

/**
 * @brief Deletes a previously generated slot.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] id The ID of the slot to delete.
 *
 * @return Tvg_Result enumeration.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded or the slot ID is invalid.
 * @retval TVG_RESULT_NOT_SUPPORTED The Lottie Animation is not supported.
 *
 * @note This function should be paired with gen.
 * @see tvg_lottie_animation_gen_slot()
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_del_slot(Tvg_Animation animation, uint32_t id);

/**
 * @brief Specifies a segment by marker.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] marker The name of the segment marker.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 * @retval TVG_RESULT_INVALID_ARGUMENT When the given @p marker is invalid.
 * @retval TVG_RESULT_NOT_SUPPORTED The Lottie Animation is not supported.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_set_marker(Tvg_Animation animation, const char* marker);

/**
 * @brief Gets the marker count of the animation.
 *
 * @param[in] animation The Lottie animation object.
 * @param[out] cnt The count value of the markers.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT In case a @c nullptr is passed as the argument.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_get_markers_cnt(Tvg_Animation animation, uint32_t* cnt);

/**
 * @deprecated see tvg_lottie_animation_get_marker_info()
 */
TVG_API TVG_DEPRECATED Tvg_Result tvg_lottie_animation_get_marker(Tvg_Animation animation, uint32_t idx, const char** name);

/**
 * @brief Retrieves marker information by index.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] idx The zero-based index of the animation marker.
 * @param[out] name Pointer to receive the marker name.
 *                  Pass @c nullptr if the value is not required.
 * @param[out] begin Pointer to receive the marker's starting frame.
 *                   Pass @c nullptr if the value is not required.
 * @param[out] end Pointer to receive the marker's ending frame.
 *                 Pass @c nullptr if the value is not required.
 *
 * @retval TVG_RESULT_INVALID_ARGUMENT if @p idx is out of range.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 *
 * @see tvg_lottie_animation_get_markers_cnt()
 * @since 1.1
 */
TVG_API Tvg_Result tvg_lottie_animation_get_marker_info(Tvg_Animation animation, uint32_t idx, const char** name, float* begin, float* end);

/**
 * @brief Interpolates between two frames over a specified duration.
 *
 * This method performs tweening, a process of generating intermediate frame
 * between @p from and @p to based on the given @p progress.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] from The start frame number of the interpolation.
 * @param[in] to The end frame number of the interpolation.
 * @param[in] progress The current progress of the interpolation (range: 0.0 to 1.0).
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION In case the animation is not loaded.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_tween(Tvg_Animation animation, float from, float to, float progress);

/**
 * @brief Sets the target frame for dynamic tweening.
 *
 * This method starts a dynamic interpolation from the current animation frame
 * toward @p to. Use tvg_lottie_animation_tween_go() to update the interpolation progress.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] to The target frame number of the interpolation.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the animation is not loaded.
 *
 * @note The dynamic tweening set by this method is discarded when @ref tvg_animation_set_frame()
 *       or @ref tvg_lottie_animation_tween() is called.
 *
 * @see tvg_lottie_animation_tween_go()
 * @note Experimental API
 */
TVG_API Tvg_Result tvg_lottie_animation_tween_to(Tvg_Animation animation, float to);

/**
 * @brief Updates the current tween toward the target frame.
 *
 * This method advances the interpolation started by @ref tvg_lottie_animation_tween_to() using the
 * given @p progress value.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] progress The current progress of the interpolation (range: 0.0 to 1.0).
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the animation is not loaded.
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If @ref tvg_lottie_animation_tween_to() has not been called.
 *
 * @see tvg_lottie_animation_tween_to()
 * @note Experimental API
 */
TVG_API Tvg_Result tvg_lottie_animation_tween_go(Tvg_Animation animation, float progress);

/**
 * @brief Sets the quality level for Lottie effects.
 *
 * This function controls the rendering quality of effects like blur, shadows, etc.
 * Lower values prioritize performance while higher values prioritize quality.
 *
 * @param[in] animation The Lottie animation object.
 * @param[in] value The quality level (0-100). 0 represents lowest quality/best performance,
 *                  100 represents highest quality/lowest performance, default is 50.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION If the animation is not loaded.
 *
 * @note This option is used as a hint; its behavior heavily depends on the render backend.
 *
 * @since 1.0
 */
TVG_API Tvg_Result tvg_lottie_animation_set_quality(Tvg_Animation animation, uint8_t value);

/**
 * @brief Describes the current state of a Lottie audio layer.
 *
 * This structure is provided to the audio resolver callback and contains
 * the information required to synchronize audio playback with the animation
 * timeline. Applications are responsible for managing audio playback using
 * their own audio engine.
 *
 * Example:
 * @code
 * void on_audio(const Tvg_Audio_Info* info, void* data)
 * {
 *     if (info->active) {
 *         // Start or seek playback of info->src.
 *     } else {
 *         // Stop playback of info->src.
 *     }
 * }
 * @endcode
 *
 * @see tvg_lottie_animation_set_audio_resolver()
 *
 * @note Experimental API
 */
typedef struct {
    const char* src;      ///< Audio source: a file path/URL or embedded raw bytes.
    const char* mimeType; ///< MIME type string; valid when @c embedded; may be @c NULL.
    uint32_t    size;     ///< Embedded data size in bytes; valid when @c embedded.
    float       offset;   ///< Position within the audio file in seconds; valid when @c active.
    float       volume;   ///< Volume [0, 100]; valid when @c active.
    bool        active;   ///< @c true while the layer is within its playback range.
    bool        embedded; ///< @c true if @p src points to embedded audio data; @c false if it is a file path or URL.
} Tvg_Audio_Info;

/**
 * @brief Callback invoked to provide audio playback information for a Lottie animation.
 *
 * Applications can use this callback to synchronize external audio
 * playback with the animation timeline.
 *
 * @param[in] info Audio information for the current timeline state.
 * @param[in] data User data specified when registering the callback.
 *
 * @see tvg_lottie_animation_set_audio_resolver()
 * @note Experimental API.
 */
typedef void (*Tvg_Audio_Resolver)(const Tvg_Audio_Info* info, void* data);

/**
 * @brief Sets the audio resolver callback for Lottie audio layers.
 *
 * The resolver is invoked whenever the playback state of an audio layer changes.
 * It allows applications to synchronize audio playback with the animation timeline.
 *
 * @param[in] animation A Lottie animation object.
 * @param[in] resolver A user-defined callback that receives audio playback state updates.
 * @param[in] data User data passed to @p resolver.
 *
 * @retval TVG_RESULT_INSUFFICIENT_CONDITION The animation has not been loaded.
 *
 * @note To disable audio notifications, pass @c nullptr as @p resolver.
 * @note Experimental API.
 *
 * @see Tvg_Audio_Resolver
 */
TVG_API Tvg_Result tvg_lottie_animation_set_audio_resolver(Tvg_Animation animation, Tvg_Audio_Resolver resolver, void* data);

/** \} */   // end addtogroup ThorVGCapi_LottieAnimation

/** \} */   // end defgroup ThorVGCapi

#ifdef __cplusplus
}
#endif

#endif //_THORVG_CAPI_H_
