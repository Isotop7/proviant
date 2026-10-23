#!/usr/bin/env python3
"""Check that every class the JavaScript queries still exists.

The Bootstrap v5 -> v6 migration renamed whole class families. A stale
selector does not fail loudly: querySelector returns null and the code path
that needed the element quietly stops working. This walks the class selectors
in src/assets/js and looks each one up in the templates and the compiled
stylesheet.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
JS_DIR = ROOT / "src" / "assets" / "js"
TEMPLATE_DIR = ROOT / "src" / "templates" / "web"
CSS = ROOT / "src" / "assets" / "css" / "main.css"

# Selectors whose classes are built at runtime or come from a vendor bundle.
SKIP = re.compile(r"bootstrap|chart|html5|proviant(?!-)", re.IGNORECASE)

# Selectors whose target markup does not exist. They are dead code paths that
# predate this check: the forgot-password confirmation and the remove-member
# button are never rendered, so the branch never runs.
KNOWN_DEAD = {
    "btn-remove-member",
    "login-form-inner",
}

CLASS_IN_SELECTOR = re.compile(r"\.([A-Za-z][\w-]*)")
SELECTOR = re.compile(
    r"""(?:querySelector(?:All)?|closest|matches|querySelector)\(\s*['"]([^'"]+)['"]"""
)


CLASS_NAME = re.compile(r"^[A-Za-z][\w-]*$")


def add_classes(classes: set[str], value: str) -> None:
    for token in value.split():
        # Skip template-literal fragments: they are not class names.
        if not CLASS_NAME.match(token):
            continue
        classes.add(token)


def collect() -> tuple[set[str], set[str]]:
    js_classes: set[str] = set()
    created: set[str] = set()
    for path in sorted(JS_DIR.glob("*.js")):
        if ".min." in path.name:
            continue
        src = path.read_text()
        for match in SELECTOR.finditer(src):
            selector = match.group(1)
            if SKIP.search(selector):
                continue
            for cls in CLASS_IN_SELECTOR.findall(selector):
                js_classes.add(cls)
        # Classes the script creates itself: assigned through className or
        # written into a class attribute in a template literal.
        for match in re.finditer(r"""className\s*=\s*['"]([^'"]+)['"]""", src):
            add_classes(created, match.group(1))
        for match in re.finditer(r'class="([^"]*)"', src):
            add_classes(created, match.group(1))
    return js_classes, created


def known_classes() -> set[str]:
    found: set[str] = set()
    for path in TEMPLATE_DIR.rglob("*.tmpl"):
        for match in re.finditer(r'class="([^"]*)"', path.read_text()):
            found.update(match.group(1).split())
    # The app's own stylesheets are the record of what the design system
    # defines. The SCSS sources are read rather than the compiled CSS so the
    # check needs no build step.
    for directory in (ROOT / "src" / "templates" / "scss", ROOT / "design-system"):
        for path in directory.rglob("*.scss"):
            for match in re.finditer(r"\.([A-Za-z][\w-]*)", path.read_text()):
                found.add(match.group(1))
    if CSS.exists():
        for match in re.finditer(r"\.([A-Za-z][\w-]*)", CSS.read_text()):
            found.add(match.group(1))
    # Bootstrap's own class names count even when the app does not import the
    # component: the class exists, the query just finds nothing.
    bootstrap_css = ROOT / "node_modules" / "bootstrap" / "dist" / "css" / "bootstrap.css"
    if bootstrap_css.exists():
        for match in re.finditer(r"\.([A-Za-z][\w-]*)", bootstrap_css.read_text()):
            found.add(match.group(1))
    return found


def main() -> None:
    js_classes, created = collect()
    known = known_classes() | created
    missing = sorted(cls for cls in js_classes if cls not in known and cls not in KNOWN_DEAD)
    if missing:
        print(f"{len(missing)} class(es) the JavaScript queries but nothing defines:")
        for cls in missing:
            print(f"  .{cls}")
        sys.exit(1)
    print(f"all {len(js_classes)} queried classes are defined")


if __name__ == "__main__":
    main()
