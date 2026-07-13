# Capture CHANAKYA screenshots into docs/screenshots/.
import os, sys
from playwright.sync_api import sync_playwright

OUT = r"C:\Projects\SEBI\CHANAKYA\docs\screenshots"
os.makedirs(OUT, exist_ok=True)
BASE = "http://localhost:3000"
VW = {"width": 1440, "height": 900}

BANNER_IDS = ["/", "/register", "/amendments", "/evidence", "/review", "/policy", "/audit", "/feed", "/regulatory-feed"]
SEED = "window.localStorage.setItem('chanakya.welcomed','1');" + \
       "".join(f"window.localStorage.setItem('chanakya.banner.{i}','1');" for i in BANNER_IDS)

def shot(page, name):
    path = os.path.join(OUT, name)
    page.screenshot(path=path)
    print(f"  saved {name} ({os.path.getsize(path)} bytes)")

def click_text(page, text, timeout=8000):
    page.get_by_role("button", name=text, exact=False).first.click(timeout=timeout)

with sync_playwright() as p:
    b = p.chromium.launch(headless=True)
    ctx = b.new_context(viewport=VW, device_scale_factor=2)
    ctx.add_init_script(SEED)
    pg = ctx.new_page()

    def go(url, wait=1600):
        pg.goto(BASE + url, wait_until="networkidle", timeout=30000)
        pg.wait_for_timeout(wait)

    # ---- data routes (need backend) ----
    plain = [("overview.png", "/"), ("register.png", "/register"),
             ("evidence.png", "/evidence"), ("review.png", "/review"),
             ("audit.png", "/audit"), ("feed.png", "/feed")]
    for name, url in plain:
        try:
            go(url, 2200)
            shot(pg, name)
        except Exception as e:
            print(f"  FAIL {name}: {repr(e)[:120]}")

    # overview graph view
    try:
        pg.add_init_script("window.sessionStorage.setItem('chanakya.overview.view','graph')")
        go("/", 1000)
        pg.evaluate("window.sessionStorage.setItem('chanakya.overview.view','graph')")
        go("/", 2600)
        shot(pg, "overview-graph.png")
    except Exception as e:
        print(f"  FAIL overview-graph.png: {repr(e)[:120]}")

    # policy: click check-rule
    try:
        go("/policy", 2000)
        try: click_text(pg, "Check this rule against your firm", 5000); pg.wait_for_timeout(1200)
        except Exception: pass
        shot(pg, "policy.png")
    except Exception as e:
        print(f"  FAIL policy.png: {repr(e)[:120]}")

    # amendments: compute blast radius
    try:
        go("/amendments", 2000)
        try: click_text(pg, "Compute blast radius", 5000); pg.wait_for_timeout(2500)
        except Exception: pass
        shot(pg, "blast-radius.png")
    except Exception as e:
        print(f"  FAIL blast-radius.png: {repr(e)[:120]}")

    # ---- simulation (backend-free) ----
    try:
        go("/regulatory-feed", 1500)
        shot(pg, "sim-inbox.png")
        seq = ["Review & Process", "View clause diff", "Extract obligations", "Update obligation graph", "Compute blast radius"]
        for t in seq:
            try: click_text(pg, t, 9000); pg.wait_for_timeout(1400)
            except Exception as e: print(f"    sim click '{t}' fail: {repr(e)[:80]}")
        pg.wait_for_timeout(1500); shot(pg, "sim-blast.png")
        for t in ["Generate workflows", "Send for human approval"]:
            try: click_text(pg, t, 9000); pg.wait_for_timeout(1400)
            except Exception as e: print(f"    sim click '{t}' fail: {repr(e)[:80]}")
        shot(pg, "sim-approval.png")
        try: click_text(pg, "Approve", 9000); pg.wait_for_timeout(1600)
        except Exception as e: print(f"    sim Approve fail: {repr(e)[:80]}")
        for t in ["Collect evidence", "Assemble audit pack"]:
            try: click_text(pg, t, 9000); pg.wait_for_timeout(1600)
            except Exception as e: print(f"    sim click '{t}' fail: {repr(e)[:80]}")
        pg.wait_for_timeout(800); shot(pg, "sim-audit.png")
    except Exception as e:
        print(f"  FAIL sim: {repr(e)[:120]}")

    b.close()
print("DONE")
