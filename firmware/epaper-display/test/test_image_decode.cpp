#include <unity.h>
#include <stdint.h>
#include <string.h>
#include <stdlib.h>

// ---------------------------------------------------------------------------
// Minimal PNG header validation logic extracted for host-side testing.
// Mirrors the checks in main.cpp without pulling in Arduino or PNGdec.
// ---------------------------------------------------------------------------

static const uint8_t PNG_SIG[8] = {0x89,'P','N','G','\r','\n',0x1a,'\n'};

typedef struct {
    int valid;
    int width;
    int height;
} PngInfo;

// Parse just the PNG signature + IHDR chunk to extract dimensions.
static PngInfo parse_png_header(const uint8_t *data, size_t len) {
    PngInfo info = {0, 0, 0};
    if (len < 24) return info;
    if (memcmp(data, PNG_SIG, 8) != 0) return info;
    // IHDR starts at byte 8; chunk length (4) + "IHDR" (4) + width (4) + height (4)
    // width at offset 16, height at offset 20
    info.width  = (int)((data[16] << 24) | (data[17] << 16) | (data[18] << 8) | data[19]);
    info.height = (int)((data[20] << 24) | (data[21] << 16) | (data[22] << 8) | data[23]);
    info.valid  = 1;
    return info;
}

static int check_dimensions(const uint8_t *data, size_t len, int req_w, int req_h) {
    PngInfo info = parse_png_header(data, len);
    if (!info.valid) return -1;  // not a PNG
    if (info.width != req_w || info.height != req_h) return -2;  // wrong size
    return 0;
}

// ---------------------------------------------------------------------------
// Minimal valid PNG: 800×480 with correct signature + IHDR
// (Not a full decodeable PNG — just enough for header parsing tests.)
// ---------------------------------------------------------------------------
static uint8_t make_ihdr_byte(int shift) {
    return 0;  // placeholder; dimensions are injected below
}

static void fill_png_header(uint8_t *buf, int w, int h) {
    memcpy(buf, PNG_SIG, 8);
    // chunk length = 13
    buf[8]  = 0; buf[9]  = 0; buf[10] = 0; buf[11] = 13;
    // chunk type = IHDR
    buf[12] = 'I'; buf[13] = 'H'; buf[14] = 'D'; buf[15] = 'R';
    // width
    buf[16] = (w >> 24) & 0xFF; buf[17] = (w >> 16) & 0xFF;
    buf[18] = (w >>  8) & 0xFF; buf[19] =  w        & 0xFF;
    // height
    buf[20] = (h >> 24) & 0xFF; buf[21] = (h >> 16) & 0xFF;
    buf[22] = (h >>  8) & 0xFF; buf[23] =  h        & 0xFF;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void test_valid_800x480_accepted(void) {
    uint8_t buf[32] = {0};
    fill_png_header(buf, 800, 480);
    int rc = check_dimensions(buf, sizeof(buf), 800, 480);
    TEST_ASSERT_EQUAL_INT(0, rc);
}

void test_wrong_dimensions_rejected(void) {
    uint8_t buf[32] = {0};
    fill_png_header(buf, 640, 480);
    int rc = check_dimensions(buf, sizeof(buf), 800, 480);
    TEST_ASSERT_EQUAL_INT(-2, rc);
}

void test_non_png_rejected(void) {
    uint8_t buf[32] = {0xFF, 0xD8, 0xFF, 0xE0};  // JPEG magic
    int rc = check_dimensions(buf, sizeof(buf), 800, 480);
    TEST_ASSERT_EQUAL_INT(-1, rc);
}

void test_too_short_rejected(void) {
    uint8_t buf[4] = {0x89, 'P', 'N', 'G'};
    int rc = check_dimensions(buf, sizeof(buf), 800, 480);
    TEST_ASSERT_EQUAL_INT(-1, rc);
}

void test_800x479_rejected(void) {
    uint8_t buf[32] = {0};
    fill_png_header(buf, 800, 479);
    int rc = check_dimensions(buf, sizeof(buf), 800, 480);
    TEST_ASSERT_EQUAL_INT(-2, rc);
}

// ---------------------------------------------------------------------------

void setUp(void) {}
void tearDown(void) {}

int main(void) {
    UNITY_BEGIN();
    RUN_TEST(test_valid_800x480_accepted);
    RUN_TEST(test_wrong_dimensions_rejected);
    RUN_TEST(test_non_png_rejected);
    RUN_TEST(test_too_short_rejected);
    RUN_TEST(test_800x479_rejected);
    return UNITY_END();
}
