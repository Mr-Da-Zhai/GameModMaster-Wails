// Generate a 512x512 appicon.png: teal gradient rounded bg + gamepad silhouette
const zlib = require('zlib');
const fs = require('fs');

const W = 512, H = 512, R = 112;
const lerp = (a, b, t) => Math.round(a + (b - a) * t);
const bg = t => [lerp(45, 14, t), lerp(212, 165, t), lerp(191, 233, t)];

function inRoundRect(x, y, w, h, r) {
  if (x < r && y < r) return Math.hypot(r - x, r - y) <= r;
  if (x > w - r && y < r) return Math.hypot(x - (w - r), r - y) <= r;
  if (x < r && y > h - r) return Math.hypot(r - x, y - (h - r)) <= r;
  if (x > w - r && y > h - r) return Math.hypot(x - (w - r), y - (h - r)) <= r;
  return true;
}

function isController(x, y) {
  const cx = W / 2, cy = H / 2 + 30, bw = 300, bh = 150;
  const x1 = cx - bw / 2, x2 = cx + bw / 2, y1 = cy - bh / 2, y2 = cy + bh / 2;
  if (x < x1 - 30 || x > x2 + 30 || y < y1 || y > y2) return false;
  const inGrip = (x < x1 + 10 && y > y2 - 50) || (x > x2 - 10 && y > y2 - 50);
  const inBody = x >= x1 && x <= x2 && y >= y1 && y <= y2;
  return inBody || inGrip;
}

const raw = Buffer.alloc((W * 4 + 1) * H);
for (let y = 0; y < H; y++) {
  raw[y * (W * 4 + 1)] = 0;
  for (let x = 0; x < W; x++) {
    let r, g, b, a = 255;
    if (!inRoundRect(x, y, W, H, R)) { a = 0; r = g = b = 0; }
    else {
      const t = (x + y) / (W + H);
      [r, g, b] = bg(t);
      if (isController(x, y)) { r = 15; g = 23; b = 42; }
      if (Math.hypot(x - 200, y - (H / 2 + 30)) <= 26) { r = 45; g = 212; b = 191; }
      const rcx = 312, rcy = H / 2 + 30;
      const dots = [[0, -30], [0, 30], [-30, 0], [30, 0]];
      for (const [dx, dy] of dots) {
        if (Math.hypot(x - (rcx + dx), y - (rcy + dy)) <= 14) { r = 45; g = 212; b = 191; }
      }
    }
    const off = y * (W * 4 + 1) + 1 + x * 4;
    raw[off] = r; raw[off + 1] = g; raw[off + 2] = b; raw[off + 3] = a;
  }
}

function crc32(buf) {
  let c = ~0;
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i];
    for (let k = 0; k < 8; k++) c = (c >>> 1) ^ (0xEDB88320 & -(c & 1));
  }
  return (~c) >>> 0;
}
function chunk(type, data) {
  const t = Buffer.from(type);
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length, 0);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([t, data])), 0);
  return Buffer.concat([len, t, data, crc]);
}

const sig = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
const ihdr = Buffer.alloc(13);
ihdr.writeUInt32BE(W, 0); ihdr.writeUInt32BE(H, 4);
ihdr[8] = 8; ihdr[9] = 6;
const idat = zlib.deflateSync(raw, { level: 9 });
const png = Buffer.concat([sig, chunk('IHDR', ihdr), chunk('IDAT', idat), chunk('IEND', Buffer.alloc(0))]);

fs.writeFileSync(process.argv[2], png);
console.log('生成', png.length, 'bytes ->', process.argv[2]);
