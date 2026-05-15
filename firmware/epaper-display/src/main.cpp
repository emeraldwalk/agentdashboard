#include <Arduino.h>
#include <SPI.h>
#include <WiFi.h>
#include <GxEPD2_BW.h>
#include <PNGdec.h>

// --- Pin assignments (confirmed from Handy4ndy/Handy-reTerminal-E1001) ---
#define EPD_SCK_PIN  7
#define EPD_MOSI_PIN 9
#define EPD_CS_PIN   10
#define EPD_DC_PIN   11
#define EPD_RES_PIN  12
#define EPD_BUSY_PIN 13

// --- Display ---
SPIClass hspi(FSPI);
GxEPD2_BW<GxEPD2_750_GDEY075T7, GxEPD2_750_GDEY075T7::HEIGHT> display(
    GxEPD2_750_GDEY075T7(EPD_CS_PIN, EPD_DC_PIN, EPD_RES_PIN, EPD_BUSY_PIN));

// --- TCP server ---
WiFiServer server(80);

// --- Image buffer ---
static const size_t MAX_IMG_BYTES = 400 * 1024;
static uint8_t *imgBuf = nullptr;

// --- Display dimensions ---
static const int EPD_W = 800;
static const int EPD_H = 480;

// --- Persistent 1-bit framebuffer (0=white, 1=black) ---
static uint8_t framebuf[EPD_W * EPD_H / 8];

// --- PNG decode state ---
static PNG png;
static int blitX = 0;
static int blitY = 0;

// ---------------------------------------------------------------------------
// PNG row callback
// ---------------------------------------------------------------------------
static int pngDraw(PNGDRAW *pDraw) {
    uint16_t rowBuf[EPD_W];
    png.getLineAsRGB565(pDraw, rowBuf, PNG_RGB565_BIG_ENDIAN, 0xFFFF);

    int dstY = blitY + pDraw->y;
    if (dstY < 0 || dstY >= EPD_H) return 1;

    for (int sx = 0; sx < pDraw->iWidth; sx++) {
        int dstX = blitX + sx;
        if (dstX < 0 || dstX >= EPD_W) continue;
        uint16_t px = rowBuf[sx];
        uint8_t r = (px >> 11) & 0x1F;
        uint8_t g = (px >> 5)  & 0x3F;
        uint8_t b =  px        & 0x1F;
        uint8_t luma = (r * 8 + g * 4 + b * 8) / 3;
        int idx    = (dstY * EPD_W + dstX);
        int byteIdx = idx / 8;
        int bitIdx  = 7 - (idx % 8);
        if (luma >= 128) {
            framebuf[byteIdx] &= ~(1 << bitIdx);  // white
        } else {
            framebuf[byteIdx] |=  (1 << bitIdx);  // black
        }
    }
    return 1;
}

// ---------------------------------------------------------------------------
// Render framebuf to display
// ---------------------------------------------------------------------------
static void renderToDisplay() {
    display.setFullWindow();
    display.firstPage();
    do {
        display.drawBitmap(0, 0, framebuf, EPD_W, EPD_H, GxEPD_BLACK);
    } while (display.nextPage());
    Serial.println("[IMG]  Render complete");
}

// ---------------------------------------------------------------------------
// Parse HTTP request line and headers from client.
// Returns method, path, and Content-Length. Reads up to end of headers.
// ---------------------------------------------------------------------------
static bool parseRequest(WiFiClient &client, String &method, String &path, int &contentLength) {
    contentLength = 0;
    unsigned long start = millis();

    // Read request line
    String requestLine = "";
    while (millis() - start < 5000) {
        if (client.available()) {
            char c = client.read();
            if (c == '\n') break;
            if (c != '\r') requestLine += c;
        }
    }

    int sp1 = requestLine.indexOf(' ');
    int sp2 = requestLine.lastIndexOf(' ');
    if (sp1 < 0 || sp2 <= sp1) return false;
    method = requestLine.substring(0, sp1);
    path   = requestLine.substring(sp1 + 1, sp2);

    // Read headers
    String line = "";
    while (millis() - start < 5000) {
        if (!client.available()) { delay(1); continue; }
        char c = client.read();
        if (c == '\n') {
            line.trim();
            if (line.length() == 0) break;  // blank line = end of headers
            String lower = line;
            lower.toLowerCase();
            if (lower.startsWith("content-length:")) {
                contentLength = line.substring(15).toInt();
            }
            line = "";
        } else if (c != '\r') {
            line += c;
        }
    }
    return true;
}

// ---------------------------------------------------------------------------
// Read exactly `len` bytes from client into buf. Returns bytes read.
// ---------------------------------------------------------------------------
static size_t readAll(WiFiClient &client, uint8_t *buf, size_t len) {
    size_t total = 0;
    unsigned long start = millis();
    while (total < len && millis() - start < 10000) {
        if (client.available()) {
            int n = client.read(buf + total, len - total);
            if (n > 0) total += n;
        } else {
            delay(1);
        }
    }
    return total;
}

// ---------------------------------------------------------------------------
// Handle one HTTP request
// ---------------------------------------------------------------------------
static void handleClient(WiFiClient &client) {
    String method, path;
    int contentLength = 0;

    if (!parseRequest(client, method, path, contentLength)) {
        client.print("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n");
        return;
    }

    Serial.printf("[HTTP] %s %s (body: %d)\n", method.c_str(), path.c_str(), contentLength);

    if (method == "POST" && path == "/clear") {
        memset(framebuf, 0x00, sizeof(framebuf));
        client.print("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK");
        Serial.println("[IMG]  Screen cleared");
        renderToDisplay();
        return;
    }

    if (method == "POST" && path == "/image") {
        if (contentLength <= 0 || contentLength > (int)MAX_IMG_BYTES) {
            client.print("HTTP/1.1 400 Bad Request\r\nContent-Length: 8\r\n\r\nBad size");
            return;
        }

        size_t got = readAll(client, imgBuf, contentLength);
        Serial.printf("[IMG]  Received %u bytes\n", got);

        if ((int)got != contentLength) {
            client.print("HTTP/1.1 400 Bad Request\r\nContent-Length: 10\r\n\r\nShort read");
            return;
        }

        // Parse optional position headers — not available with raw TCP, default to 0,0
        int originX = 0, originY = 0;

        int rc = png.openRAM(imgBuf, (int)got, pngDraw);
        if (rc != PNG_SUCCESS) {
            Serial.printf("[ERR]  PNG open failed: %d\n", rc);
            client.print("HTTP/1.1 400 Bad Request\r\nContent-Length: 9\r\n\r\nBad image");
            return;
        }

        int w = png.getWidth();
        int h = png.getHeight();
        Serial.printf("[IMG]  Decoded %dx%d at (%d,%d)\n", w, h, originX, originY);

        if (originX < 0 || originY < 0 || originX + w > EPD_W || originY + h > EPD_H) {
            png.close();
            client.print("HTTP/1.1 400 Bad Request\r\nContent-Length: 13\r\n\r\nOut of bounds");
            return;
        }

        blitX = originX;
        blitY = originY;
        png.decode(nullptr, 0);
        png.close();

        client.print("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK");
        renderToDisplay();
        return;
    }

    client.print("HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n");
}

// ---------------------------------------------------------------------------
// setup / loop
// ---------------------------------------------------------------------------
void setup() {
    Serial.begin(115200);
    while (!Serial && millis() < 3000) delay(10);
    delay(500);
    Serial.println("[BOOT] ePaper display firmware starting");

    imgBuf = (uint8_t *)malloc(MAX_IMG_BYTES);
    if (!imgBuf) Serial.println("[ERR]  imgBuf malloc failed");

    memset(framebuf, 0x00, sizeof(framebuf));

    Serial.println("[BOOT] Initializing SPI");
    hspi.begin(EPD_SCK_PIN, -1, EPD_MOSI_PIN, -1);
    Serial.println("[BOOT] Initializing display");
    display.init(115200, true, 2, false, hspi, SPISettings(2000000, MSBFIRST, SPI_MODE0));
    Serial.println("[BOOT] Display init done");

    display.setFullWindow();
    display.firstPage();
    do { display.fillScreen(GxEPD_WHITE); } while (display.nextPage());

    WiFi.mode(WIFI_STA);
    WiFi.begin(WIFI_SSID, WIFI_PASS);
    Serial.println("[WIFI] Connecting...");
    unsigned long start = millis();
    while (WiFi.status() != WL_CONNECTED && millis() - start < 10000) {
        delay(200);
        Serial.print(".");
    }
    Serial.println();

    if (WiFi.status() == WL_CONNECTED) {
        Serial.printf("[WIFI] Connected: %s\n", WiFi.localIP().toString().c_str());
    } else {
        Serial.println("[WIFI] Failed — rebooting");
        delay(1000);
        ESP.restart();
    }

    server.begin();
    Serial.println("[HTTP] Listening on :80");
}

void loop() {
    WiFiClient client = server.available();
    if (client) {
        handleClient(client);
        client.stop();
    }
}
