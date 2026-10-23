#!/usr/bin/env python3
"""Fill the pack size of the seeded demo products from Open Food Facts.

The seed file states, per product, the barcode, the name and the number of
packs on hand. The size of one pack is not in the file: it is read from the
barcode, because the barcode is what knows it. A product the data source does
not know keeps an empty pack size, and the pack size backfill fills it in
later.

Usage:
    python3 scripts/seed-products.py [csv-path] [--dry-run]

The file is rewritten in place. The columns the script owns are the pack count
and the pack size; every other column is left exactly as it was.
"""

import argparse
import csv
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

DEFAULT_CSV = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "seed-products.csv")
API = "https://world.openfoodfacts.org/api/v2/product/{barcode}?fields=quantity"
USER_AGENT = "proviant-seed-script/1.0 (https://github.com/Isotop7/proviant)"
REQUEST_DELAY_SECONDS = 2.0
RATE_LIMIT_RETRIES = 4
# A quantity of the form "6 x 150 g" states a count of packs and the size of
# one of them, sometimes with the total behind an equals sign ("6 x 150 g =
# 900 g"). Every other form is the size of a single pack.
COUNT_PATTERN = re.compile(r"^(\d+)\s*[x×]\s*(.+?)(?:\s*=\s*.+)?$")


def fetch_quantity(barcode):
    """Return the quantity the data source states for a barcode, or ""."""
    for attempt in range(RATE_LIMIT_RETRIES):
        request = urllib.request.Request(API.format(barcode=barcode), headers={"User-Agent": USER_AGENT})
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                payload = json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as error:
            if error.code == 404:
                return ""
            if error.code == 429:
                # The data source throttles the read tier: back off and try
                # again rather than losing the row.
                wait = REQUEST_DELAY_SECONDS * (2 ** (attempt + 1))
                print(f"  ~ {barcode}: throttled, retrying in {wait:.0f}s", file=sys.stderr)
                time.sleep(wait)
                continue
            print(f"  ! {barcode}: {error}", file=sys.stderr)
            return ""
        except (urllib.error.URLError, TimeoutError, ValueError) as error:
            print(f"  ! {barcode}: {error}", file=sys.stderr)
            return ""
        product = payload.get("product") or {}
        return (product.get("quantity") or "").strip()
    return ""


def split_quantity(quantity):
    """Split a stated quantity into a pack count and the size of one pack."""
    if not quantity:
        return 1, ""
    match = COUNT_PATTERN.match(quantity)
    if match:
        return int(match.group(1)), match.group(2).strip()
    return 1, quantity


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("csv_path", nargs="?", default=DEFAULT_CSV)
    parser.add_argument("--dry-run", action="store_true", help="report the changes without writing the file")
    args = parser.parse_args()

    path = os.path.abspath(args.csv_path)
    with open(path, newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle, delimiter=";")
        fieldnames = reader.fieldnames
        rows = list(reader)

    if not fieldnames or "barcode" not in fieldnames or "pack_size" not in fieldnames:
        print(f"Error: {path} has no barcode and pack_size columns.", file=sys.stderr)
        return 1

    changed = 0
    for index, row in enumerate(rows):
        barcode = (row.get("barcode") or "").strip()
        if not barcode:
            continue
        count, pack_size = split_quantity(fetch_quantity(barcode))
        if pack_size == row.get("pack_size") and str(count) == (row.get("quantity") or "").strip():
            continue
        row["quantity"] = str(count)
        row["pack_size"] = pack_size
        changed += 1
        print(f"  {barcode}: {count} x {pack_size}")
        if index < len(rows) - 1:
            time.sleep(REQUEST_DELAY_SECONDS)

    if args.dry_run:
        print(f"\n{changed} rows would change. Nothing written.")
        return 0

    with open(path, "w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames, delimiter=";")
        writer.writeheader()
        writer.writerows(rows)

    print(f"\n{changed} of {len(rows)} rows filled. {path} written.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
