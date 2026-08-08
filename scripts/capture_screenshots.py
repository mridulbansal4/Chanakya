# Capture CHANAKYA screenshots into docs/screenshots/.
#
#   1. start the app:  .\dev.ps1     (backend :8080 + web :3000)
#   2. py scripts/capture_screenshots.py
#
# Every image in the root README.md is produced by this script against a live
# backend - nothing is mocked or hand-edited.
import os
from playwright.sync_api import sync_playwright

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs", "screenshots")
os.makedirs(OUT, exist_ok=True)

BASE = "http://localhost:3000"
VW = {"width": 1440, "height": 900}

# Routes that render a first-visit banner / welcome modal. Pre-dismissing them
# keeps the captures showing the product rather than its onboarding.
ROUTES = ["/", "/ingest", "/review", "/workflows", "/evidence", "/company",
          "/register", "/regulatory-feed", "/amendments", "/policy", "/audit", "/feed"]
SEED = ("window.localStorage.setItem('chanakya.welcomed','1');"
        + "".join(f"window.localStorage.setItem('chanakya.banner.{r}','1');" for r in ROUTES))


def shot(page, name, full=False):
    path = os.path.join(OUT, name)
    page.screenshot(path=path, full_page=full)
    print(f"  saved {name} ({os.path.getsize(path) // 1024} KB)")


def click_text(page, text, timeout=8000):
    page.get_by_role("button", name=text, exact=False).first.click(timeout=timeout)


with sync_playwright() as p:
    b = p.chromium.launch(headless=True)
    ctx = b.new_context(viewport=VW, device_scale_factor=2)
    ctx.add_init_script(SEED)
    pg = ctx.new_page()

    def go(url, wait=2200):
        pg.goto(BASE + url, wait_until="networkidle", timeout=45000)
        pg.wait_for_timeout(wait)

    # ---- plain routes: navigate, settle, capture --------------------------
    plain = [
        ("overview.png", "/"),
        ("ingest.png", "/ingest"),
        ("review.png", "/review"),
        ("workflows.png", "/workflows"),
        ("evidence.png", "/evidence"),
        ("company.png", "/company"),
        ("register.png", "/register"),
        ("regulatory-feed.png", "/regulatory-feed"),
        ("audit.png", "/audit"),
        ("feed.png", "/feed"),
    ]
    for name, url in plain:
        try:
            go(url)
            shot(pg, name)
        except Exception as e:
            print(f"  FAIL {name}: {repr(e)[:140]}")

    # ---- overview, graph view --------------------------------------------
    try:
        go("/", 1000)
        pg.evaluate("window.sessionStorage.setItem('chanakya.overview.view','graph')")
        go("/", 3000)
        shot(pg, "overview-graph.png")
    except Exception as e:
        print(f"  FAIL overview-graph.png: {repr(e)[:140]}")

    # ---- policy: compile the signed obligation to Rego, then evaluate it,
    #      so the capture shows the deterministic verdict rather than a CTA.
    try:
        go("/policy")
        for label in ("Compile to Automated Check",
                      "Check this rule against your firm",
                      "Evaluate"):
            try:
                click_text(pg, label, 6000)
                pg.wait_for_timeout(2600)
            except Exception:
                continue
        shot(pg, "policy.png")
    except Exception as e:
        print(f"  FAIL policy.png: {repr(e)[:140]}")

    # ---- amendments: compute the blast radius so the graph is populated ---
    try:
        go("/amendments")
        for label in ("Compute blast radius", "Compute"):
            try:
                click_text(pg, label, 5000)
                pg.wait_for_timeout(3000)
                break
            except Exception:
                continue
        shot(pg, "blast-radius.png")
    except Exception as e:
        print(f"  FAIL blast-radius.png: {repr(e)[:140]}")

    b.close()
print("DONE")
