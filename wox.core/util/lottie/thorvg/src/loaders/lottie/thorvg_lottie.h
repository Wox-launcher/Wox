#ifndef _THORVG_LOTTIE_H_
#define _THORVG_LOTTIE_H_

#include "thorvg.h"

namespace tvg
{

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
 * animation->resolver([](const LottieAudioResolver& info, void*) {
 *     if (info.active) {
 *         // Start or seek playback of info.src.
 *     } else {
 *         // Stop playback of info.src.
 *     }
 * }, nullptr);
 * @endcode
 *
 * @see LottieAnimation::resolver()
 *
 * @note Experimental API
 */
struct LottieAudioResolver
{
    const char* src;         ///< Audio source: a file path/URL or embedded raw bytes.
    const char* mimeType;    ///< MIME type string; valid when @c embedded; may be @c nullptr.
    uint32_t    size;        ///< Embedded data size in bytes; valid when @c embedded.
    float       offset;      ///< Position within the audio file in seconds; valid when @c active.
    float       volume;      ///< Volume [0, 100]; valid when @c active.
    bool        active;      ///< @c true while the layer is within its playback range.
    bool        embedded;    ///< @c true if @p src points to embedded audio data; @c false if it is a file path or URL.
};

/**
 * @class LottieAnimation
 *
 * @brief The LottieAnimation class enables control of advanced Lottie features.
 *
 * This class extends the Animation and has additional interfaces.
 *
 * @see Animation
 * 
 * @since 0.15
 */
struct TVG_API LottieAnimation final : Animation
{
    ~LottieAnimation() override;

    /**
    * @brief Specifies a segment by marker. 
    * 
    * Markers are used to control animation playback by specifying start and end points, 
    * eliminating the need to know the exact frame numbers.
    * Generally, markers are designated at the design level, 
    * meaning the callers must know the marker name in advance to use it.
    *
    * @param[in] marker The name of the segment marker.
    *
    * @retval Result::InsufficientCondition If the animation is not loaded.
    * @retval Result::NonSupport When it's not animatable.
    *
    * @note If a @c marker is specified, the previously set segment will be disregarded.
    * @note Set @c nullptr to reset the specified segment.
    * @see Animation::segment(float begin, float end)
    * @since 1.0
    */
    Result segment(const char* marker) noexcept;

    /**
     * @brief Interpolates between two frames over a specified duration.
     *
     * This method performs tweening, a process of generating intermediate frame
     * between @p from and @p to based on the given @p progress.
     *
     * @param[in] from The start frame number of the interpolation.
     * @param[in] to The end frame number of the interpolation.
     * @param[in] progress The current progress of the interpolation (range: 0.0 to 1.0).
     *
     * @retval Result::InsufficientCondition In case the animation is not loaded.
     *
     * @see tweenTo(float)
     *
     * @since 1.0
     */
    Result tween(float from, float to, float progress) noexcept;

    /**
     * @brief Sets the target frame for dynamic tweening.
     *
     * This method starts a dynamic interpolation from the current animation frame
     * toward @p to. Use tween(float progress) to update the interpolation progress.
     *
     * @param[in] to The target frame number of the interpolation.
     *
     * @retval Result::InsufficientCondition If the animation is not loaded.
     *
     * @note The dynamic tweening set by this method is discarded when @ref Animation::frame()
     *       or @ref LottieAnimation::tween(float,float,float) is called.
     *
     * @see tween(float)
     * @note Experimental API
     */
    Result tweenTo(float to) noexcept;

    /**
     * @brief Updates the current tween toward the target frame.
     *
     * This method advances the interpolation started by tweenTo() using the
     * given @p progress value.
     *
     * @param[in] progress The current progress of the interpolation (range: 0.0 to 1.0).
     *
     * @retval Result::InsufficientCondition If the animation is not loaded.
     * @retval Result::InsufficientCondition If @ref tweenTo() has not been called.
     *
     * @see tweenTo()
     * @note Experimental API
     */
    Result tween(float progress) noexcept;

    /**
     * @brief Gets the marker count of the animation.
     *
     * @retval The count of the markers, zero if there is no marker.
     * 
     * @see LottieAnimation::marker()
     * @since 1.0
     */
    uint32_t markersCnt() noexcept;

    /**
     * @deprecated see marker(uint32_t, float*, float*)
     */
    TVG_DEPRECATED const char* marker(uint32_t idx) noexcept;

    /**
     * @brief Retrieves the name and frame range of a marker by index.
     *
     * @param[in] idx The zero-based index of the animation marker.
     * @param[out] begin Pointer to receive the marker's starting frame.
     *                   Pass @c nullptr if the value is not required.
     * @param[out] end Pointer to receive the marker's ending frame.
     *                 Pass @c nullptr if the value is not required.
     *
     * @return The name of the marker on success, or @c nullptr otherwise.
     *
     * @see LottieAnimation::markersCnt()
     * @since 1.1
     */
    const char* marker(uint32_t idx, float* begin, float* end) noexcept;

    /**
     * @brief Creates a new slot based on the given Lottie slot data.
     *
     * This function parses the provided JSON-formatted slot data and generates
     * a new slot for animation control. The returned slot ID can be used to apply
     * or delete the slot later.
     *
     * @param[in] slot A JSON string representing the Lottie slot data.
     *
     * @return A unique, non-zero slot ID on success. Returns @c 0 if the slot generation fails.
     *
     * @see apply(uint32_t id)
     * @see del(uint32_t id)
     *
     * @since 1.0
     */
    uint32_t gen(const char* slot) noexcept;

    /**
     * @brief Applies a previously generated slot to the animation.
     *
     * This function applies the animation parameters defined by a slot.
     * If the provided slot ID is 0, all previously applied slots will be reset.
     *
     * @param[in] id The ID of the slot to apply. Use 0 to reset all slots.
     *
     * @retval Result::InvalidArguments If the animation is not loaded or the slot ID is invalid.
     *
     * @see gen(const char* slot)
     *
     * @since 1.0
     */
    Result apply(uint32_t id) noexcept;

    /**
     * @brief Deletes a previously generated slot.
     *
     * This function removes a slot by its ID.
     *
     * @param[in] id The ID of the slot to delete. Retrieve the ID from gen().
     *
     * @retval Result::InvalidArguments If the animation is not loaded or the slot ID is invalid.
     *
     * @note This function should be paired with gen.
     * @see gen(const char* slot)
     *
     * @since 1.0
     */
    Result del(uint32_t id) noexcept;

    /**
     * @brief Sets the quality level for Lottie effects.
     *
     * This function controls the rendering quality of effects like blur, shadows, etc.
     * Lower values prioritize performance while higher values prioritize quality.
     *
     * @param[in] value The quality level (0-100). 0 represents lowest quality/best performance,
     *                  100 represents highest quality/lowest performance, default is 50.
     *
     * @retval Result::InsufficientCondition If the animation is not loaded.
     *
     * @since 1.0
     */
    Result quality(uint8_t value) noexcept;

    /**
     * @brief Sets the audio resolver callback for Lottie audio layers.
     *
     * The resolver is invoked whenever the playback state of an audio layer changes.
     * It allows applications to synchronize audio playback with the animation timeline.
     *
     * @param[in] func A user-defined callback that receives audio playback state updates.
     * @param[in] data User data passed to @p func.
     *
     * @retval Result::InsufficientCondition The animation has not been loaded.
     *
     * @note To disable audio notifications, pass @c nullptr as @p func.
     *
     * @see LottieAudioResolver
     *
     * @note Experimental API
     */
    Result resolver(std::function<void(const LottieAudioResolver& info, void* data)> func, void* data) noexcept;

    /**
     * @brief Creates a new LottieAnimation object.
     *
     * @return A new LottieAnimation object.
     *
     * @since 0.15
     */
    static LottieAnimation* gen() noexcept;

    _TVG_DECLARE_PRIVATE(LottieAnimation);
};

} //namespace

#endif //_THORVG_LOTTIE_H_
