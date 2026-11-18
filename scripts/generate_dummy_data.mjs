// Generates data/dummy_activities.csv: one week (Mon-Sun) of daily
// activity for 100 executives (IDs 1001-1100).
//
// Usage: node scripts/generate_dummy_data.mjs [output.csv]
//
// Columns: date,ex_id,login_time,target_count
//  - login_time is IST "HH:MM"; empty string means no login that day.
//  - Weekdays: ~74% log in between 09:00-17:00 with target >= 25
//    (eligible), the rest fall short in different ways.
//  - Weekend: ~40% eligible, so supervisors (70% rule) and the district
//    coordinator (85% rule) are visibly NOT paid on Sat/Sun.
import { writeFileSync } from "node:fs";

const OUT = process.argv[2] ?? "data/dummy_activities.csv";
const EXEC_IDS = Array.from({ length: 100 }, (_, i) => 1001 + i);

// deterministic PRNG so the data is reproducible
function mulberry32(seed) {
  return function () {
    seed |= 0; seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = mulberry32(20261117);
const randInt = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
const pad = (n) => String(n).padStart(2, "0");

// current week Monday..Sunday (as of generation time)
const now = new Date();
const dow = (now.getDay() + 6) % 7; // Mon=0
const monday = new Date(now);
monday.setDate(now.getDate() - dow);

const rows = [];
for (let d = 0; d < 7; d++) {
  const day = new Date(monday);
  day.setDate(monday.getDate() + d);
  const date = `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
  const isWeekend = d >= 5;
  const eligibleRate = isWeekend ? 0.4 : 0.74;

  for (const exId of EXEC_IDS) {
    const roll = rnd();
    if (roll < eligibleRate) {
      // eligible: in-window login, target 26-60
      const h = randInt(9, 16), m = randInt(0, 59);
      rows.push(`${date},${exId},${pad(h)}:${pad(m)},${randInt(26, 60)}`);
    } else if (roll < eligibleRate + 0.11) {
      // logged in but outside 9-17 window
      const late = rnd() < 0.5;
      const h = late ? randInt(17, 21) : randInt(5, 8), m = randInt(0, 59);
      rows.push(`${date},${exId},${pad(h)}:${pad(m)},${randInt(30, 60)}`);
    } else if (roll < eligibleRate + 0.21) {
      // in-window login but target < 25
      const h = randInt(9, 16), m = randInt(0, 59);
      rows.push(`${date},${exId},${pad(h)}:${pad(m)},${randInt(0, 24)}`);
    } else {
      // no login at all
      rows.push(`${date},${exId},,${randInt(0, 20)}`);
    }
  }
}
writeFileSync(OUT, "date,ex_id,login_time,target_count\n" + rows.join("\n") + "\n");
console.log(`wrote ${rows.length} rows to ${OUT}`);
