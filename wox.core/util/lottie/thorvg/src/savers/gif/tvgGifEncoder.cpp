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


//
// gif.h
// by Charlie Tangora
// Public domain.
// Email me : ctangora -at- gmail -dot- com
//
// This file offers a simple, very limited way to create animated GIFs directly in code.
//
// Those looking for particular cleverness are likely to be disappointed; it's pretty
// much a straight-ahead implementation of the GIF format with optional Floyd-Steinberg
// dithering. (It does at least use delta encoding - only the changed portions of each
// frame are saved.)
//
// So resulting files are often quite large. The hope is that it will be handy nonetheless
// as a quick and easily-integrated way for programs to spit out animations.
//
// Only RGBA8 is currently supported as an input format. (The alpha is ignored.)
//
// USAGE:
// Create a GifWriter struct. Pass it to GifBegin() to initialize and write the header.
// Pass subsequent frames to GifWriteFrame().
// Finally, call GifEnd() to close the file handle and free memory.
//

#include "tvgMath.h"
#include "tvgGifEncoder.h"


#define TRANSPARENT_IDX 0
#define TRANSPARENT_THRESHOLD 127
#define BIT_DEPTH 8


// Simple structure to write out the LZW-compressed portion of the image
// one bit at a time
typedef struct
{
    uint8_t bitIndex;  // how many bits in the partial byte written so far
    uint8_t byte;      // current partial byte

    uint32_t chunkIndex;
    uint8_t chunk[256];   // bytes are written in here until we have 256 of them, then written to the file
} GifBitStatus;


// The LZW dictionary is a 256-ary tree constructed as the file is encoded,
// this is one node
typedef struct
{
    uint16_t m_next[256];
} GifLzwNode;


/************************************************************************/
/* Internal Class Implementation                                        */
/************************************************************************/

// walks the k-d tree to pick the palette entry for a desired color.
// Takes as in/out parameters the current best color and its error -
// only changes them if it finds a better color in its subtree.
// this is the major hotspot in the code at the moment.
static void _getClosestPaletteColor( GifPalette* pPal, int r, int g, int b, int* bestInd, int* bestDiff, int treeRoot )
{
    // base case, reached the bottom of the tree
    if (treeRoot > (1 << BIT_DEPTH) - 1) {
        int ind = treeRoot-(1 << BIT_DEPTH);
        if(ind == TRANSPARENT_IDX) return;

        // check whether this color is better than the current winner
        int r_err = r - ((int32_t)pPal->r[ind]);
        int g_err = g - ((int32_t)pPal->g[ind]);
        int b_err = b - ((int32_t)pPal->b[ind]);
        int diff = abs(r_err)+ abs(g_err) + abs(b_err);

        if(diff < *bestDiff) {
            *bestInd = ind;
            *bestDiff = diff;
        }
        return;
    }

    // take the appropriate color (r, g, or b) for this node of the k-d tree
    int comps[3] = {r, g, b};

    int splitComp = comps[pPal->treeSplitElt[treeRoot]];

    int splitPos = pPal->treeSplit[treeRoot];
    if (splitPos > splitComp) {
        // check the left subtree
        _getClosestPaletteColor(pPal, r, g, b, bestInd, bestDiff, treeRoot*2);
        if (*bestDiff > splitPos - splitComp) {
            // cannot prove there's not a better value in the right subtree, check that too
            _getClosestPaletteColor(pPal, r, g, b, bestInd, bestDiff, treeRoot*2+1);
        }
    } else {
        _getClosestPaletteColor(pPal, r, g, b, bestInd, bestDiff, treeRoot*2+1);
        if (*bestDiff > splitComp - splitPos) {
            _getClosestPaletteColor(pPal, r, g, b, bestInd, bestDiff, treeRoot*2);
        }
    }
}


static void _swapPixels(uint8_t* image, int pixA, int pixB)
{
    uint8_t rA = image[pixA*4];
    uint8_t gA = image[pixA*4+1];
    uint8_t bA = image[pixA*4+2];
    uint8_t aA = image[pixA*4+3];

    uint8_t rB = image[pixB*4];
    uint8_t gB = image[pixB*4+1];
    uint8_t bB = image[pixB*4+2];
    uint8_t aB = image[pixA*4+3];

    image[pixA*4] = rB;
    image[pixA*4+1] = gB;
    image[pixA*4+2] = bB;
    image[pixA*4+3] = aB;

    image[pixB*4] = rA;
    image[pixB*4+1] = gA;
    image[pixB*4+2] = bA;
    image[pixB*4+3] = aA;
}


// just the partition operation from quicksort
static int _partition(uint8_t* image, const int left, const int right, const int elt, int pivotIndex)
{
    const int pivotValue = image[(pivotIndex)*4+elt];
    _swapPixels(image, pivotIndex, right-1);
    int storeIndex = left;
    bool split = 0;

    for (int ii = left; ii < right - 1; ++ii) {
        int arrayVal = image[ii*4+elt];
        if(arrayVal < pivotValue) {
            _swapPixels(image, ii, storeIndex);
            ++storeIndex;
        } else if(arrayVal == pivotValue) {
            if (split) {
                _swapPixels(image, ii, storeIndex);
                ++storeIndex;
            }
            split = !split;
        }
    }
    _swapPixels(image, storeIndex, right-1);
    return storeIndex;
}


// Perform an incomplete sort, finding all elements above and below the desired median
static void _partitionByMedian(uint8_t* image, int left, int right, int com, int neededCenter)
{
    if (left < right-1) {
        int pivotIndex = left + (right-left)/2;
        pivotIndex = _partition(image, left, right, com, pivotIndex);

        // Only "sort" the section of the array that contains the median
        if(pivotIndex > neededCenter) _partitionByMedian(image, left, pivotIndex, com, neededCenter);
        if(pivotIndex < neededCenter) _partitionByMedian(image, pivotIndex+1, right, com, neededCenter);
    }
}


// Builds a palette by creating a balanced k-d tree of all pixels in the image
static void _splitPalette(uint8_t* image, int numPixels, int firstElt, int lastElt, int splitElt, int splitDist, int treeNode, GifPalette* pal)
{
    if(lastElt <= firstElt || numPixels == 0) return;

    // base case, bottom of the tree
    if (lastElt == firstElt + 1) {
        // otherwise, take the average of all colors in this subcube
        uint64_t r=0, g=0, b=0;
        for(int ii=0; ii<numPixels; ++ii) {
            r += image[ii*4+0];
            g += image[ii*4+1];
            b += image[ii*4+2];
        }

        r += (uint64_t)numPixels / 2;  // round to nearest
        g += (uint64_t)numPixels / 2;
        b += (uint64_t)numPixels / 2;

        r /= (uint64_t)numPixels;
        g /= (uint64_t)numPixels;
        b /= (uint64_t)numPixels;

        pal->r[firstElt] = (uint8_t)r;
        pal->g[firstElt] = (uint8_t)g;
        pal->b[firstElt] = (uint8_t)b;
        return;
    }

    // Find the axis with the largest range
    int minR = 255, maxR = 0;
    int minG = 255, maxG = 0;
    int minB = 255, maxB = 0;

    for (int ii=0; ii<numPixels; ++ii) {
        int r = image[ii*4+0];
        int g = image[ii*4+1];
        int b = image[ii*4+2];

        if(r > maxR) maxR = r;
        if(r < minR) minR = r;

        if(g > maxG) maxG = g;
        if(g < minG) minG = g;

        if(b > maxB) maxB = b;
        if(b < minB) minB = b;
    }

    int rRange = maxR - minR;
    int gRange = maxG - minG;
    int bRange = maxB - minB;

    // and split along that axis. (incidentally, this means this isn't a "proper" k-d tree but I don't know what else to call it)
    int splitCom = 1;
    if (bRange > gRange) splitCom = 2;
    if (rRange > bRange && rRange > gRange) splitCom = 0;

    int subPixelsA = numPixels * (splitElt - firstElt) / (lastElt - firstElt);
    int subPixelsB = numPixels-subPixelsA;

    _partitionByMedian(image, 0, numPixels, splitCom, subPixelsA);

    pal->treeSplitElt[treeNode] = (uint8_t)splitCom;
    pal->treeSplit[treeNode] = image[subPixelsA*4+splitCom];

    _splitPalette(image, subPixelsA, firstElt, splitElt, splitElt-splitDist, splitDist/2, treeNode*2, pal);
    _splitPalette(image+subPixelsA*4, subPixelsB, splitElt, lastElt,  splitElt+splitDist, splitDist/2, treeNode*2+1, pal);
}


// Finds all pixels that have changed from the previous image and
// moves them to the from of the buffer.
// This allows us to build a palette optimized for the colors of the
// changed pixels only.
static int _pickChangedPixels(const uint8_t* lastFrame, uint8_t* frame, int numPixels, bool transparent)
{
    int numChanged = 0;
    uint8_t* writeIter = frame;

    for (int ii=0; ii < numPixels; ++ii) {
        if (frame[3] >= TRANSPARENT_THRESHOLD) {
            if (transparent || (lastFrame[0] != frame[0] || lastFrame[1] != frame[1] || lastFrame[2] != frame[2])) {
                writeIter[0] = frame[0];
                writeIter[1] = frame[1];
                writeIter[2] = frame[2];
                ++numChanged;
                writeIter += 4;
            }
        }
        lastFrame += 4;
        frame += 4;
    }

    return numChanged;
}


// Creates a palette by placing all the image pixels in a k-d tree and then averaging the blocks at the bottom.
// This is known as the "modified median split" technique
static void _makePalette(GifWriter* writer, const uint8_t* lastFrame, const uint8_t* nextFrame, uint32_t width, uint32_t height, int bitDepth, bool transparent)
{
    auto& pal = writer->pal;

    size_t imageSize = (size_t)(width * height * 4 * sizeof(uint8_t));
    memcpy(writer->tmpImage, nextFrame, imageSize);

    int numPixels = (int)(width * height);
    if (lastFrame) numPixels = _pickChangedPixels(lastFrame, writer->tmpImage, numPixels, transparent);

    const int lastElt = 1 << bitDepth;
    const int splitElt = lastElt/2;
    const int splitDist = splitElt/2;

    _splitPalette(writer->tmpImage, numPixels, 1, lastElt, splitElt, splitDist, 1, &pal);

    // add the bottom node for the transparency index
    pal.treeSplit[1 << (bitDepth-1)] = 0;
    pal.treeSplitElt[1 << (bitDepth-1)] = 0;
    pal.r[0] = pal.g[0] = pal.b[0] = 0;
}


void _palettizePixel(const uint8_t* nextFrame, uint8_t* outFrame, GifPalette* pPal)
{
    int32_t bestDiff = 1000000;
    int32_t bestInd = 1;
    _getClosestPaletteColor(pPal, nextFrame[0], nextFrame[1], nextFrame[2], &bestInd, &bestDiff, 1);

    // Write the resulting color to the output buffer
    outFrame[0] = pPal->r[bestInd];
    outFrame[1] = pPal->g[bestInd];
    outFrame[2] = pPal->b[bestInd];
    outFrame[3] = (uint8_t)bestInd;
}


// Picks palette colors for the image using simple threshholding, no dithering
static void _thresholdImage(GifWriter* writer, const uint8_t* lastFrame, const uint8_t* nextFrame,  uint32_t width, uint32_t height, bool transparent)
{
    auto outFrame = writer->oldImage;
    uint32_t numPixels = width*height;

    if (transparent) {
        for (uint32_t ii = 0; ii < numPixels; ++ii) {
            if (nextFrame[3] < TRANSPARENT_THRESHOLD) {
                outFrame[0] = 0;
                outFrame[1] = 0;
                outFrame[2] = 0;
                outFrame[3] = TRANSPARENT_IDX;
            } else {
                _palettizePixel(nextFrame, outFrame, &writer->pal);
            }
            if (lastFrame) lastFrame += 4;
            outFrame += 4;
            nextFrame += 4;
        }
    } else {
        for (uint32_t ii = 0; ii < numPixels; ++ii) {
            // if a previous color is available, and it matches the current color,
            // set the pixel to transparent
            if(lastFrame && lastFrame[0] == nextFrame[0] && lastFrame[1] == nextFrame[1] && lastFrame[2] == nextFrame[2]) {
                outFrame[0] = lastFrame[0];
                outFrame[1] = lastFrame[1];
                outFrame[2] = lastFrame[2];
                outFrame[3] = TRANSPARENT_IDX;
            } else {
                _palettizePixel(nextFrame, outFrame, &writer->pal);
            }
            if (lastFrame) lastFrame += 4;
            outFrame += 4;
            nextFrame += 4;
        }
    }
}


// insert a single bit
static void _writeBit(GifBitStatus* stat, uint32_t bit)
{
    bit = bit & 1;
    bit = bit << stat->bitIndex;
    stat->byte |= bit;

    ++stat->bitIndex;
    if (stat->bitIndex > 7) {
        // move the newly-finished byte to the chunk buffer
        stat->chunk[stat->chunkIndex++] = stat->byte;
        // and start a new byte
        stat->bitIndex = 0;
        stat->byte = 0;
    }
}


// write all bytes so far to the file
static void _writeChunk(FILE* f, GifBitStatus* stat)
{
    fputc((int)stat->chunkIndex, f);
    fwrite(stat->chunk, 1, stat->chunkIndex, f);

    stat->bitIndex = 0;
    stat->byte = 0;
    stat->chunkIndex = 0;
}


static void _writeCode(FILE* f, GifBitStatus* stat, uint32_t code, uint32_t length)
{
    for (uint32_t ii = 0; ii < length; ++ii) {
        _writeBit(stat, code);
        code = code >> 1;
        if (stat->chunkIndex == 255) _writeChunk(f, stat);
    }
}


// write a 256-color (8-bit) image palette to the file
static void _writePalette(const GifPalette* pPal, FILE* f)
{
    fputc(0, f);  // first color: transparency
    fputc(0, f);
    fputc(0, f);

    for (int ii = 1; ii < (1 << BIT_DEPTH); ++ii) {
        uint32_t r = pPal->r[ii];
        uint32_t g = pPal->g[ii];
        uint32_t b = pPal->b[ii];

        fputc((int)r, f);
        fputc((int)g, f);
        fputc((int)b, f);
    }
}


// write the image header, LZW-compress and write out the image
static void _writeLzwImage(GifWriter* writer, uint32_t width, uint32_t height, uint32_t delay, bool transparent)
{
    auto f = writer->f;
    auto image = writer->oldImage;

    // graphics control extension
    fputc(0x21, f);
    fputc(0xf9, f);
    fputc(0x04, f);
    fputc((transparent ? 0x09 : 0x05), f);  //clear prev frame or not.
    fputc(delay & 0xff, f);
    fputc((delay >> 8) & 0xff, f);
    fputc(TRANSPARENT_IDX, f); // transparent color index
    fputc(0, f);

    fputc(0x2c, f); // image descriptor block

    // corner of image (left, top) in canvas space
    fputc(0, f);
    fputc(0, f);
    fputc(0, f);
    fputc(0, f);

    fputc(width & 0xff, f);          // width and height of image
    fputc((width >> 8) & 0xff, f);
    fputc(height & 0xff, f);
    fputc((height >> 8) & 0xff, f);

    //fputc(0, f); // no local color table, no transparency
    //fputc(0x80, f); // no local color table, but transparency

    fputc(0x80 + BIT_DEPTH - 1, f); // local color table present, 2 ^ bitDepth entries
    _writePalette(&writer->pal, f);

    const int minCodeSize = BIT_DEPTH;
    const uint32_t clearCode = 1 << BIT_DEPTH;

    fputc(minCodeSize, f); // min code size 8 bits

    GifLzwNode* codetree = tvg::malloc<GifLzwNode>(sizeof(GifLzwNode)*4096);

    memset(codetree, 0, sizeof(GifLzwNode)*4096);
    int32_t curCode = -1;
    uint32_t codeSize = (uint32_t)minCodeSize + 1;
    uint32_t maxCode = clearCode+1;

    GifBitStatus stat;
    stat.byte = 0;
    stat.bitIndex = 0;
    stat.chunkIndex = 0;

    _writeCode(f, &stat, clearCode, codeSize);  // start with a fresh LZW dictionary

    for (uint32_t yy = 0; yy < height; ++yy) {
        for (uint32_t xx=0; xx<width; ++xx) {
            // top-left origin
            uint8_t nextValue = image[(yy*width+xx)*4+3];

            // "loser mode" - no compression, every single code is followed immediately by a clear
            //WriteCode( f, stat, nextValue, codeSize );
            //WriteCode( f, stat, 256, codeSize );

            if (curCode < 0) {
                // first value in a new run
                curCode = nextValue;
            } else if (codetree[curCode].m_next[nextValue]) {
                // current run already in the dictionary
                curCode = codetree[curCode].m_next[nextValue];
            } else {
                // finish the current run, write a code
                _writeCode(f, &stat, (uint32_t)curCode, codeSize);

                // insert the new run into the dictionary
                codetree[curCode].m_next[nextValue] = (uint16_t)++maxCode;

                if (maxCode >= (1ul << codeSize)) {
                    // dictionary entry count has broken a size barrier,
                    // we need more bits for codes
                    codeSize++;
                }
                if (maxCode == 4095) {
                    // the dictionary is full, clear it out and begin anew
                    _writeCode(f, &stat, clearCode, codeSize); // clear tree

                    memset(codetree, 0, sizeof(GifLzwNode)*4096);
                    codeSize = (uint32_t)(minCodeSize + 1);
                    maxCode = clearCode+1;
                }

                curCode = nextValue;
            }
        }
    }

    // compression footer
    _writeCode(f, &stat, (uint32_t)curCode, codeSize);
    _writeCode(f, &stat, clearCode, codeSize);
    _writeCode(f, &stat, clearCode + 1, (uint32_t)minCodeSize + 1);

    // write out the last partial chunk
    while (stat.bitIndex) _writeBit(&stat, 0);
    if (stat.chunkIndex) _writeChunk(f, &stat);

    fputc(0, f); // image block terminator

    tvg::free(codetree);
}


/************************************************************************/
/* External Class Implementation                                        */
/************************************************************************/


bool gifBegin(GifWriter* writer, const char* filename, uint32_t width, uint32_t height, uint32_t delay)
{
#if defined(_MSC_VER) && (_MSC_VER >= 1400)
	writer->f = 0;
    fopen_s(&writer->f, filename, "wb");
#else
    writer->f = fopen(filename, "wb");
#endif
    if (!writer->f) return false;

    writer->firstFrame = true;

    // allocate
    writer->oldImage = tvg::malloc<uint8_t>(width*height*4);
    writer->tmpImage = tvg::malloc<uint8_t>(width*height*4);

    fputs("GIF89a", writer->f);

    // screen descriptor
    fputc(width & 0xff, writer->f);
    fputc((width >> 8) & 0xff, writer->f);
    fputc(height & 0xff, writer->f);
    fputc((height >> 8) & 0xff, writer->f);

    fputc(0xf0, writer->f);  // there is an unsorted global color table of 2 entries
    fputc(0, writer->f);     // background color
    fputc(0, writer->f);     // pixels are square (we need to specify this because it's 1989)

    // now the "global" palette (really just a dummy palette)
    // color 0: black
    fputc(0, writer->f);
    fputc(0, writer->f);
    fputc(0, writer->f);
    // color 1: also black
    fputc(0, writer->f);
    fputc(0, writer->f);
    fputc(0, writer->f);

    if(delay != 0) {
        // animation header
        fputc(0x21, writer->f); // extension
        fputc(0xff, writer->f); // application specific
        fputc(11, writer->f); // length 11
        fputs("NETSCAPE2.0", writer->f); // yes, really
        fputc(3, writer->f); // 3 bytes of NETSCAPE2.0 data

        fputc(1, writer->f); // JUST BECAUSE
        fputc(0, writer->f); // loop infinitely (byte 0)
        fputc(0, writer->f); // loop infinitely (byte 1)

        fputc(0, writer->f); // block terminator
    }

    return true;
}


bool gifWriteFrame(GifWriter* writer, const uint8_t* image, uint32_t width, uint32_t height, uint32_t delay, bool transparent)
{
    if (!writer->f) return false;

    const uint8_t* oldImage = writer->firstFrame? NULL : writer->oldImage;
    writer->firstFrame = false;

    _makePalette(writer, oldImage, image, width, height, 8, transparent);
    _thresholdImage(writer, oldImage, image, width, height, transparent);
    _writeLzwImage(writer, width, height, delay, transparent);

    return true;
}


bool gifEnd(GifWriter* writer)
{
    if (!writer->f) return false;

    fputc(0x3b, writer->f); // end of file
    fclose(writer->f);
    tvg::free(writer->oldImage);
    tvg::free(writer->tmpImage);

    writer->f = NULL;
    writer->oldImage = NULL;

    return true;
}
