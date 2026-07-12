# -*- coding: utf-8 -*-
"""
Generate the Alpha Wealth Advisors demo document bundle for the SETU/CHANAKYA
Regulatory Amendment Simulation. Fictional COMPANY documents grounded in the REAL
SEBI IA Master Circular and the SEBI MITC Circular (SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/
2025/19, 17 Feb 2025). Times New Roman, corporate layout, revision history,
approval signatures, Confidential footer.
"""
import os, shutil, csv
from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm
from reportlab.lib import colors
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_RIGHT
from reportlab.platypus import (
    BaseDocTemplate, PageTemplate, Frame, Paragraph, Spacer, Table, TableStyle,
    PageBreak, HRFlowable, KeepTogether,
)

OUT = r"C:\Projects\SEBI\CHANAKYA\Documents"
os.makedirs(OUT, exist_ok=True)

COMPANY = "Alpha Wealth Advisors Pvt. Ltd."
SEBI_REG = "INA000000001"
ADDRESS = "Mumbai, Maharashtra, India"
CLIENT = "Rahul Sharma"
MITC_REF = "SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19"
MITC_DATE = "17 February 2025"
DEADLINE = "30 June 2025"

INK = colors.HexColor("#141414")
DIM = colors.HexColor("#555555")
LINE = colors.HexColor("#BBBBBB")
LIGHT = colors.HexColor("#EFEFEF")

# ---- styles -----------------------------------------------------------------
ss = getSampleStyleSheet()
def S(name, **kw):
    base = kw.pop("parent", ss["Normal"])
    return ParagraphStyle(name, parent=base, **kw)

BODY = S("body", fontName="Times-Roman", fontSize=11, leading=15.5, alignment=TA_JUSTIFY, spaceAfter=6)
BODYL = S("bodyl", parent=BODY, alignment=0)
TITLE = S("title", fontName="Times-Bold", fontSize=22, leading=26, alignment=TA_CENTER, textColor=INK, spaceAfter=4)
SUBTITLE = S("subtitle", fontName="Times-Roman", fontSize=12.5, leading=16, alignment=TA_CENTER, textColor=DIM)
H1 = S("h1", fontName="Times-Bold", fontSize=14.5, leading=18, textColor=INK, spaceBefore=14, spaceAfter=6)
H2 = S("h2", fontName="Times-Bold", fontSize=12, leading=15, textColor=INK, spaceBefore=10, spaceAfter=4)
SMALL = S("small", fontName="Times-Roman", fontSize=9, leading=12, textColor=DIM)
SMALLC = S("smallc", parent=SMALL, alignment=TA_CENTER)
CELL = S("cell", fontName="Times-Roman", fontSize=9.5, leading=12.5)
CELLB = S("cellb", parent=CELL, fontName="Times-Bold")
EYEBROW = S("eyebrow", fontName="Times-Bold", fontSize=9, leading=11, textColor=DIM, alignment=TA_CENTER)

def P(t, s=BODY): return Paragraph(t, s)
def rule(w=1, c=LINE): return HRFlowable(width="100%", thickness=w, color=c, spaceBefore=4, spaceAfter=8)
def gap(h=6): return Spacer(1, h)

def kv_table(rows, col0=42*mm):
    data = [[P(k, CELLB), P(v, CELL)] for k, v in rows]
    t = Table(data, colWidths=[col0, None])
    t.setStyle(TableStyle([
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("LINEBELOW", (0,0), (-1,-2), 0.4, LINE),
        ("TOPPADDING", (0,0), (-1,-1), 4), ("BOTTOMPADDING", (0,0), (-1,-1), 4),
        ("LEFTPADDING", (0,0), (-1,-1), 2), ("RIGHTPADDING", (0,0), (-1,-1), 6),
    ]))
    return t

def grid(headers, rows, widths=None, header_bg=INK, header_fg=colors.white):
    data = [[P(h, S("th", fontName="Times-Bold", fontSize=9.5, textColor=header_fg)) for h in headers]]
    for r in rows:
        data.append([P(str(c), CELL) for c in r])
    t = Table(data, colWidths=widths, repeatRows=1)
    t.setStyle(TableStyle([
        ("BACKGROUND", (0,0), (-1,0), header_bg),
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("GRID", (0,0), (-1,-1), 0.4, LINE),
        ("ROWBACKGROUNDS", (0,1), (-1,-1), [colors.white, colors.HexColor("#F7F7F7")]),
        ("TOPPADDING", (0,0), (-1,-1), 5), ("BOTTOMPADDING", (0,0), (-1,-1), 5),
        ("LEFTPADDING", (0,0), (-1,-1), 6), ("RIGHTPADDING", (0,0), (-1,-1), 6),
    ]))
    return t

def revision_history(rows):
    out = [Paragraph("Revision History", H2),
           grid(["Version", "Date", "Author", "Summary of Change"], rows,
                widths=[20*mm, 26*mm, 34*mm, None])]
    return out

def approval_block(rows):
    out = [Paragraph("Document Approval", H2),
           grid(["Role", "Name", "Signature", "Date"], rows,
                widths=[46*mm, 44*mm, 40*mm, None])]
    return out

def clause(num, title, body_paras):
    items = [Paragraph(f"{num}. {title}", H2)]
    for b in body_paras:
        items.append(P(b))
    return KeepTogether(items)

# ---- page furniture (header rule + confidential footer + page no) -----------
def make_footer(doc_code):
    def _footer(canvas, doc):
        canvas.saveState()
        w, h = A4
        # top hairline with company + doc code
        canvas.setFont("Times-Bold", 8.5); canvas.setFillColor(DIM)
        canvas.drawString(20*mm, h-14*mm, COMPANY.upper())
        canvas.setFont("Times-Roman", 8.5)
        canvas.drawRightString(w-20*mm, h-14*mm, doc_code)
        canvas.setStrokeColor(LINE); canvas.setLineWidth(0.5)
        canvas.line(20*mm, h-16*mm, w-20*mm, h-16*mm)
        # footer
        canvas.line(20*mm, 15*mm, w-20*mm, 15*mm)
        canvas.setFont("Times-Roman", 8); canvas.setFillColor(DIM)
        canvas.drawString(20*mm, 11*mm, f"CONFIDENTIAL  •  {COMPANY}")
        canvas.drawCentredString(w/2, 11*mm, f"SEBI Reg. {SEBI_REG}")
        canvas.drawRightString(w-20*mm, 11*mm, f"Page {doc.page}")
        canvas.restoreState()
    return _footer

def build(filename, story, doc_code):
    path = os.path.join(OUT, filename)
    doc = BaseDocTemplate(path, pagesize=A4,
                          leftMargin=20*mm, rightMargin=20*mm,
                          topMargin=22*mm, bottomMargin=20*mm,
                          title=filename.replace(".pdf",""), author=f"{COMPANY} — Compliance Department")
    frame = Frame(doc.leftMargin, doc.bottomMargin, doc.width, doc.height, id="f")
    doc.addPageTemplates([PageTemplate(id="main", frames=[frame], onPage=make_footer(doc_code))])
    doc.build(story)
    print("wrote", path)

def cover(title, subtitle, meta_rows, doc_code):
    el = [gap(40), Paragraph(COMPANY, S("cc", fontName="Times-Bold", fontSize=15, alignment=TA_CENTER, textColor=INK)),
          Paragraph(f"SEBI Registered Investment Adviser &nbsp;&bull;&nbsp; {SEBI_REG}", SMALLC),
          Paragraph(ADDRESS, SMALLC), gap(30), rule(1.2, INK),
          Paragraph(title, TITLE), Paragraph(subtitle, SUBTITLE), gap(6), rule(1.2, INK), gap(24),
          kv_table(meta_rows, col0=50*mm), gap(30),
          Paragraph("Prepared by the Compliance Department", SMALLC),
          Paragraph(f"Document Reference: {doc_code}", SMALLC)]
    return el

# =============================================================================
# 1 & 2 — INVESTMENT ADVISORY AGREEMENT
# =============================================================================
def agreement(version, dated, with_mitc):
    code = f"AWA/IA-AGR/{ '2025-02' if with_mitc else '2024-01' }/v{version}"
    st = []
    st += cover("Investment Advisory Agreement",
                f"Version {version}.0" + ("  —  Post-MITC Amendment" if with_mitc else "  —  Baseline"),
                [("Client Name", CLIENT), ("Adviser", COMPANY), ("SEBI Registration", SEBI_REG),
                 ("Agreement Version", f"{version}.0"), ("Effective Date", dated),
                 ("Status", "In force" if with_mitc else "Superseded by v2.0")], code)
    st.append(PageBreak())

    st.append(Paragraph("Investment Advisory Agreement", H1))
    st.append(P(f"This Investment Advisory Agreement (“Agreement”) is entered into at Mumbai on {dated} "
                f"between {COMPANY}, a company registered under the Companies Act, 2013 and registered with the "
                f"Securities and Exchange Board of India (SEBI) as a Non-Individual Investment Adviser under "
                f"Registration No. {SEBI_REG} (the “Investment Adviser”), and Mr. {CLIENT}, an individual "
                f"resident in India (the “Client”)."))
    st.append(P("The Investment Adviser and the Client are hereinafter individually referred to as a “Party” "
                "and collectively as the “Parties”."))

    st.append(Paragraph("Parties", H2))
    st.append(kv_table([("Investment Adviser", f"{COMPANY}, {ADDRESS}"),
                        ("SEBI Registration No.", SEBI_REG),
                        ("Client", f"Mr. {CLIENT}"),
                        ("Client Category", "Individual — Retail")]))

    st += [
        clause(1, "Definitions", [
            "“Advisory Services” means investment advice provided by the Investment Adviser in accordance with the SEBI "
            "(Investment Advisers) Regulations, 2013 and the SEBI Master Circular for Investment Advisers.",
            "“Regulations” means the SEBI (Investment Advisers) Regulations, 2013 as amended, and all circulars, "
            "guidelines and directions issued by SEBI thereunder."]),
        clause(2, "Scope of Advisory Services", [
            "The Investment Adviser shall provide investment advice to the Client based on the Client's risk profile, "
            "financial situation and investment objectives, and shall act in a fiduciary capacity and in the best interest of the Client.",
            "The Investment Adviser shall not manage the Client's funds or securities, shall not exercise any power of attorney, "
            "and shall not guarantee or assure any return on investments.",
            "The Investment Adviser shall carry out KYC and risk profiling of the Client and shall periodically review the same."]),
        clause(3, "Fees", [
            "The Client shall pay advisory fees to the Investment Adviser in accordance with the mode and limits specified by SEBI. "
            "Fees shall be charged either as a percentage of Assets under Advice (AUA) or as a fixed fee, within the ceilings prescribed by SEBI.",
            "Fees once paid are non-refundable except on a pro-rata basis where the Agreement is terminated by the Client."]),
        clause(4, "Obligations of the Client", [
            "The Client shall provide accurate and complete information for KYC, risk profiling and suitability assessment, and shall "
            "promptly notify the Investment Adviser of any material change in such information."]),
        clause(5, "Risk Disclosure", [
            "Investments in securities are subject to market and other risks. There is no assurance or guarantee of returns. "
            "Past performance of the Investment Adviser or of any security is not indicative of future performance."]),
        clause(6, "Confidentiality", [
            "The Investment Adviser shall keep confidential all personal and financial information of the Client and shall not disclose "
            "the same except as required by law, regulation or a competent authority."]),
        clause(7, "Term and Termination", [
            "This Agreement shall remain in force until terminated by either Party by giving 30 days' written notice. On termination, the "
            "Investment Adviser shall refund the pro-rata unexpired portion of any advance fee."]),
        clause(8, "Grievance Redressal and Dispute Resolution", [
            "Any grievance may be raised with the Investment Adviser's Compliance Officer. If unresolved, the Client may escalate the "
            "grievance to SEBI through the SCORES portal, and disputes may be referred to the Online Dispute Resolution (ODR) mechanism "
            "specified by SEBI. This Agreement shall be governed by the laws of India, with jurisdiction of the courts at Mumbai."]),
    ]

    if with_mitc:
        st.append(PageBreak())
        st.append(Paragraph("Annexure A — Most Important Terms and Conditions (MITC)", H1))
        st.append(P(f"Pursuant to SEBI Circular {MITC_REF} dated {MITC_DATE}, the following standardized Most Important Terms "
                    f"and Conditions (MITC) form an integral part of this Agreement and have been read and understood by the Client."))
        mitc = [
            "The Investment Adviser is registered with SEBI; SEBI registration, certification or any empanelment does not guarantee the performance of the Investment Adviser or provide any assurance of returns.",
            "The Investment Adviser provides only investment advice; it does not manage funds or securities on behalf of the Client and shall not seek any power of attorney over the Client's assets.",
            "The Investment Adviser shall not assure or guarantee any returns, profits or capital protection under any circumstances.",
            "Advisory fees are payable only in the modes and within the limits specified by SEBI. The Client shall not make any payment towards products or services not covered by SEBI's fee provisions.",
            "Investments are subject to market risk; the Client should read all offer/scheme related documents carefully before investing.",
            "The Investment Adviser shall disclose all conflicts of interest and shall act in a fiduciary capacity in the best interest of the Client.",
            "The Client's KYC, risk profile and suitability must be established before any advice is acted upon.",
            "The Client's information shall be kept confidential and used only for the purpose of providing advisory services.",
            "The Client may terminate the engagement and is entitled to a pro-rata refund of any advance fee for the unexpired period.",
            "Grievances may be lodged with the Investment Adviser and escalated to SEBI via the SCORES portal; disputes may be resolved through the SEBI Online Dispute Resolution (ODR) mechanism.",
            "The Client should deal only with SEBI-registered Investment Advisers and may verify the registration on the SEBI website.",
        ]
        st.append(grid(["#", "Most Important Term or Condition"],
                       [[i+1, m] for i, m in enumerate(mitc)], widths=[12*mm, None]))
        st.append(gap(10))
        st.append(Paragraph("Client Acknowledgement of MITC", H2))
        st.append(P("I, Mr. Rahul Sharma, confirm that I have received, read and understood the Most Important Terms and "
                    "Conditions (MITC) set out above, and that the same have been explained to me by the Investment Adviser."))
        st.append(gap(6))
        st.append(grid(["Client Name", "Acknowledged On", "Mode", "Status"],
                       [["Mr. Rahul Sharma", "28 May 2025", "Email + e-Sign", "Acknowledged"]],
                       widths=[46*mm, 34*mm, 40*mm, None]))

    st.append(PageBreak())
    if with_mitc:
        st += revision_history([
            ["1.0", "01 Jan 2024", "Compliance Dept.", "Initial execution of Investment Advisory Agreement."],
            ["2.0", "20 May 2025", "Compliance Dept.",
             f"Updated after SEBI Circular {MITC_REF} dated {MITC_DATE}: standardized MITC (Annexure A) incorporated and Client acknowledgement obtained."],
        ])
    else:
        st += revision_history([
            ["1.0", "01 Jan 2024", "Compliance Dept.", "Initial execution of Investment Advisory Agreement."],
        ])
    st.append(gap(12))
    st += approval_block([
        ["Investment Adviser (Authorised Signatory)", "A. Nair, Director", "", dated],
        ["Compliance Officer", "Priya Menon", "", dated],
        ["Client", f"Mr. {CLIENT}", "", dated],
    ])
    build(f"IA_Agreement_v{version}.pdf", st, code)

# =============================================================================
# 3 & 4 — INTERNAL COMPLIANCE POLICY
# =============================================================================
def compliance_policy(version, dated, updated):
    code = f"AWA/COMP-POL/{'2025-02' if updated else '2023-04'}/v{version}"
    st = []
    st += cover("Internal Compliance Policy",
                f"Version {version}.0" + ("  —  Post-MITC Amendment" if updated else "  —  Baseline"),
                [("Policy Owner", "Compliance Department"), ("Approving Authority", "Board of Directors"),
                 ("Version", f"{version}.0"), ("Effective Date", dated),
                 ("Applicable Regulation", "SEBI (Investment Advisers) Regulations, 2013 & IA Master Circular"),
                 ("Review Cycle", "Annual, or on regulatory change")], code)
    st.append(PageBreak())

    st.append(Paragraph("1. Purpose and Scope", H1))
    st.append(P(f"This Internal Compliance Policy sets out the framework by which {COMPANY} (“the Firm”) ensures "
                "compliance with the SEBI (Investment Advisers) Regulations, 2013, the SEBI Master Circular for Investment "
                "Advisers, and all applicable SEBI circulars and directions. It applies to all directors, employees and "
                "authorised persons of the Firm."))

    st.append(Paragraph("2. Regulatory Monitoring", H1))
    st.append(P("The Compliance Department shall continuously monitor SEBI circulars, master circulars and directions. Each new "
                "regulatory instrument shall be logged, assessed for applicability, and mapped to the Firm's obligations, controls "
                "and documents within defined timelines."))

    st.append(Paragraph("3. Client Communication", H1))
    st.append(P("All client-facing communication shall be fair, not misleading, and consistent with SEBI requirements. Advisory "
                "communications shall be recorded and retained. The Firm shall not assure or guarantee returns in any communication."))

    st.append(Paragraph("4. Documentation and Record Keeping", H1))
    st.append(P("The Firm shall maintain KYC records, risk profiles, suitability assessments, executed client agreements, advice "
                "rendered and rationale, and fee records for the period prescribed under the Regulations."))

    st.append(Paragraph("5. Compliance Review", H1))
    st.append(P("The Compliance Officer shall conduct periodic reviews of adherence to this Policy and to the Regulations, and shall "
                "report findings to the Board. Non-compliance shall be remediated through a tracked action plan."))

    st.append(Paragraph("6. Audit Preparation", H1))
    st.append(P("The Firm shall maintain an audit-ready state at all times, including a complete evidence trail linking each "
                "obligation to its source regulation, the controls that satisfy it, and the evidence of satisfaction."))

    if updated:
        st.append(PageBreak())
        st.append(Paragraph("7. Mandatory Client Notification Workflow (MITC)", H1))
        st.append(P(f"Pursuant to SEBI Circular {MITC_REF} dated {MITC_DATE}, the Firm shall operate a mandatory client "
                    "notification workflow for the standardized Most Important Terms and Conditions (MITC):"))
        st.append(grid(["Step", "Action", "Owner", "Timeline"],
                       [["1", "Incorporate standardized MITC into the client agreement (v2.0)", "Compliance", "Immediate"],
                        ["2", "Notify all existing clients of the MITC", "Client Servicing", DEADLINE],
                        ["3", "Obtain and record client acknowledgement of the MITC", "Client Servicing", DEADLINE],
                        ["4", "Retain evidence of notification and acknowledgement", "Compliance", "Ongoing"],
                        ["5", "Report completion status to the Board", "Compliance Officer", "On completion"]],
                       widths=[14*mm, None, 34*mm, 28*mm]))

        st.append(Paragraph("8. MITC Implementation", H1))
        st.append(P("New client onboarding shall include the standardized MITC as part of the agreement. Existing clients shall be "
                    f"informed of the MITC on or before {DEADLINE}. The MITC content shall not be altered from the SEBI-standardized text."))

        st.append(Paragraph("9. Evidence Retention", H1))
        st.append(P("For each client, the Firm shall retain: the version of the agreement incorporating the MITC, the notification "
                    "sent, and the client's acknowledgement. Evidence shall be linked to the corresponding obligation and clause."))

        st.append(Paragraph("10. Acknowledgement Tracking", H1))
        st.append(P("The Compliance Department shall maintain a Client Acknowledgement Register recording, for every client, whether "
                    "the MITC has been sent and acknowledged, with dates and mode, and shall escalate any pending acknowledgements."))

        st.append(Paragraph("11. Updated Compliance Responsibilities", H1))
        st.append(grid(["Function", "Responsibility"],
                       [["Compliance Officer", "Owns the MITC change, human sign-off, and Board reporting"],
                        ["Client Servicing", "Notifies clients and collects acknowledgements by " + DEADLINE],
                        ["Records", "Retains agreement v2.0, notifications and acknowledgements"],
                        ["Board", "Approves the updated policy and reviews completion"]],
                       widths=[46*mm, None]))

    st.append(PageBreak())
    rows = [["1.0", "01 Apr 2023", "Compliance Dept.", "Initial Internal Compliance Policy."]]
    if updated:
        rows.append(["2.0", "20 May 2025", "Compliance Dept.",
                     f"Updated after SEBI Circular {MITC_REF} dated {MITC_DATE}: added mandatory MITC client notification "
                     "workflow, MITC implementation, evidence retention, acknowledgement tracking and updated responsibilities."])
    st += revision_history(rows)
    st.append(gap(12))
    st += approval_block([
        ["Prepared by — Compliance Officer", "Priya Menon", "", dated],
        ["Reviewed by — Director", "A. Nair", "", dated],
        ["Approved by — Board of Directors", "Board Resolution", "", dated],
    ])
    build(f"Internal_Compliance_Policy_v{version}.pdf", st, code)

# =============================================================================
# 5 — AUDIT PACK (Reg 19(3)) — comprehensive, ~15 pages
# =============================================================================
def audit_pack():
    code = "AWA/AUDIT/2025/REG-19(3)"
    st = []
    st += cover("Compliance Audit Pack",
                "Regulatory Amendment — Most Important Terms and Conditions (MITC)",
                [("Prepared for", "Board of Directors / SEBI Inspection"),
                 ("Regulatory Event", f"{MITC_REF}"),
                 ("Circular Date", MITC_DATE), ("Compliance Deadline", DEADLINE),
                 ("Reference Regulation", "Regulation 19(3), SEBI (IA) Regulations, 2013"),
                 ("Overall Status", "Compliant"), ("Pack Version", "1.0"), ("Date of Issue", "05 June 2025")], code)
    st.append(PageBreak())

    # TOC
    st.append(Paragraph("Contents", H1))
    toc = [["1", "Executive Summary"], ["2", "Scope, Objective and Methodology"], ["3", "Regulatory Background"],
           ["4", "Regulatory Amendment"], ["5", "Affected Clauses"], ["6", "Updated Obligations"],
           ["7", "Compliance Actions"], ["8", "Control Environment and Testing"], ["9", "Human Approval"],
           ["10", "Evidence Collected"], ["11", "Client Notifications"], ["12", "Agreement Updates"],
           ["13", "Policy Updates"], ["14", "Outstanding Risks"], ["15", "Final Compliance Status and Attestation"],
           ["A", "Appendix A — Clause-to-Evidence Lineage"], ["B", "Appendix B — Document Register"]]
    st.append(grid(["Section", "Title"], toc, widths=[24*mm, None]))
    st.append(PageBreak())

    st.append(Paragraph("1. Executive Summary", H1))
    st.append(P(f"This Audit Pack documents the end-to-end compliance response of {COMPANY} (“the Firm”) to SEBI Circular "
                f"{MITC_REF} dated {MITC_DATE} on the Most Important Terms and Conditions (MITC) for Investment Advisers. The Firm, "
                "already compliant with the SEBI (Investment Advisers) Regulations, 2013 and the IA Master Circular, detected the new "
                "circular, assessed its operational impact, generated and executed the required workflows under human approval, and "
                "collected the supporting evidence — restoring a fully compliant, audit-ready state."))
    st.append(P("No obligation was actioned automatically. Every AI-proposed obligation was reviewed and approved by the Compliance "
                "Officer before any operational action was taken, consistent with the Firm's governance framework."))
    st.append(gap(4))
    st.append(grid(["Metric", "Value"],
                   [["Obligations modified", "3"], ["Obligations added", "1"], ["Agreements updated", "1 template (v1 → v2)"],
                    ["Existing clients notified", "2"], ["Acknowledgements received", "2"], ["Policies updated", "1 (v1 → v2)"],
                    ["Evidence artefacts collected", "5"], ["Human approvals", "1 (Compliance Officer)"],
                    ["Final status", "Compliant — Reg 19(3)"]],
                   widths=[70*mm, None]))
    st.append(gap(6))
    st.append(P("This pack has been prepared to a standard suitable for internal governance and for production to SEBI during "
                "inspection under Regulation 19(3) of the SEBI (Investment Advisers) Regulations, 2013. It is self-contained and "
                "each assertion is supported by a referenced evidence artefact listed in the Document Register (Appendix B)."))
    st.append(PageBreak())

    st.append(Paragraph("2. Scope, Objective and Methodology", H1))
    st.append(P("Objective. To evidence that the Firm has identified, assessed and fully implemented the requirements of SEBI "
                f"Circular {MITC_REF} dated {MITC_DATE}, and that a complete, human-approved and auditable trail exists from the "
                "regulatory text to the operational evidence."))
    st.append(P("Scope. The review covers the Firm's client agreement template, internal compliance policy, client register, "
                "client notifications and acknowledgements, and the human approval governing the change, for the period from "
                f"{MITC_DATE} to the date of this pack. It does not extend to matters unrelated to the MITC amendment."))
    st.append(Paragraph("Methodology", H2))
    st.append(grid(["Step", "Procedure"],
                   [["1. Detect", "Continuous monitoring of SEBI circulars identified the MITC circular on issue."],
                    ["2. Assess", "The circular was parsed and diffed against the Firm's in-force obligations."],
                    ["3. Extract", "Obligations were extracted as structured data and cited to their source clause."],
                    ["4. Impact", "A blast-radius analysis identified affected policies, agreements, clients and controls."],
                    ["5. Approve", "The Compliance Officer reviewed and approved the plan before any action was taken."],
                    ["6. Execute", "Agreement and policy were updated; clients notified; acknowledgements collected."],
                    ["7. Evidence", "Each action produced evidence linked to the obligation it satisfies."],
                    ["8. Attest", "Final status assessed and attested by the Compliance Officer and Director."]],
                   widths=[26*mm, None]))
    st.append(PageBreak())

    st.append(Paragraph("3. Regulatory Background", H1))
    st.append(P("The Firm is registered with SEBI as a Non-Individual Investment Adviser and operates under the SEBI (Investment "
                "Advisers) Regulations, 2013 and the SEBI Master Circular for Investment Advisers. Regulation 19 requires the "
                "maintenance of records and, under sub-regulation (3), that such records be maintained in a manner that enables "
                "verification of compliance."))
    st.append(P(f"On {MITC_DATE}, SEBI issued Circular {MITC_REF} standardizing the Most Important Terms and Conditions (MITC) that "
                "an Investment Adviser must disclose to clients in simple language. The circular requires the MITC to be shared with "
                "clients, incorporated into the engagement, and acknowledged; existing clients are to be informed on or before "
                f"{DEADLINE}. The MITC standardizes disclosures on registration status, advisory-only scope, absence of assured "
                "returns, fee limits, non-handling of client funds, risk, conflicts of interest, confidentiality, termination and "
                "grievance redressal through SCORES and the SEBI Online Dispute Resolution mechanism."))
    st.append(PageBreak())

    st.append(Paragraph("4. Regulatory Amendment", H1))
    st.append(kv_table([("Circular", MITC_REF), ("Date", MITC_DATE),
                        ("Subject", "Most Important Terms and Conditions (MITC) for Investment Advisers"),
                        ("Issued by", "SEBI — MIRSD"), ("Compliance deadline (existing clients)", DEADLINE),
                        ("Nature of change", "New disclosure + acknowledgement obligation")]))
    st.append(P("The circular standardizes the Most Important Terms and Conditions that an Investment Adviser must share with clients "
                "in simple language, requires the MITC to be incorporated in the client engagement, mandates client acknowledgement, "
                f"and requires existing clients to be informed on or before {DEADLINE}."))

    st.append(Paragraph("5. Affected Clauses", H1))
    st.append(grid(["Clause", "Description", "Change"],
                   [["MITC ¶2", "Provision of standardized MITC to clients", "Added"],
                    ["MITC ¶3", "Incorporation of MITC into client agreement", "Modified"],
                    ["MITC ¶4", "Client acknowledgement and record retention", "Modified"],
                    ["MITC ¶5", "Notification of existing clients by " + DEADLINE, "Added"]],
                   widths=[24*mm, None, 24*mm]))

    st.append(Paragraph("6. Updated Obligations", H1))
    st.append(grid(["Ref", "Obligation", "Type", "Confidence"],
                   [["MITC-1", "Provide standardized MITC and obtain acknowledgement", "New", "98%"],
                    ["3.1", "Client agreement incorporates the MITC", "Modified", "97%"],
                    ["3.4", "Retain MITC acknowledgement as record", "Modified", "96%"],
                    ["5.2", f"Inform existing clients of MITC by {DEADLINE}", "Modified", "98%"]],
                   widths=[18*mm, None, 24*mm, 24*mm]))
    st.append(PageBreak())

    st.append(Paragraph("7. Compliance Actions", H1))
    st.append(grid(["#", "Action", "Owner", "Priority", "Status"],
                   [["1", "Update agreement template (v1 → v2)", "Compliance", "Critical", "Completed"],
                    ["2", "Notify existing clients of the MITC", "Client Servicing", "Critical", "Completed"],
                    ["3", "Collect client acknowledgements", "Client Servicing", "High", "Completed"],
                    ["4", "Update internal compliance policy (v2)", "Compliance", "High", "Completed"],
                    ["5", "Generate audit report", "Compliance", "Normal", "Completed"]],
                   widths=[10*mm, None, 30*mm, 22*mm, 22*mm]))

    st.append(Paragraph("8. Control Environment and Testing", H1))
    st.append(P("The Firm's compliance controls relevant to the MITC amendment were tested for both design and operating "
                "effectiveness over the review period. All controls were found to have operated effectively; no exceptions were noted."))
    st.append(grid(["Control", "Control Objective", "Test Performed", "Result"],
                   [["C-01 Regulatory monitoring", "New SEBI circulars are detected and assessed", "Traced the MITC circular from detection to obligation extraction", "Effective"],
                    ["C-02 Change approval", "No change is enforced without human approval", "Inspected the Compliance Officer's sign-off before execution", "Effective"],
                    ["C-03 Agreement versioning", "Client agreement reflects current obligations", "Verified agreement moved v1.0 → v2.0 with MITC annexure", "Effective"],
                    ["C-04 Client notification", "Existing clients informed within the deadline", "Vouched 2 notifications sent and delivered before " + DEADLINE, "Effective"],
                    ["C-05 Acknowledgement tracking", "Client acknowledgements captured and retained", "Reconciled 2 acknowledgements to the register", "Effective"],
                    ["C-06 Evidence retention", "Each obligation linked to its evidence", "Traced clause → obligation → workflow → evidence (Appendix A)", "Effective"]],
                   widths=[36*mm, None, None, 22*mm]))
    st.append(PageBreak())

    st.append(Paragraph("9. Human Approval", H1))
    st.append(P("The AI-proposed obligations and operational plan were reviewed and approved by the Firm's Compliance Officer prior "
                "to execution. No action was enforced automatically."))
    st.append(kv_table([("Reviewer", "Priya Menon — Compliance Officer"), ("Decision", "Approved"),
                        ("Confidence", "98%"), ("Basis", f"Source clause of {MITC_REF}"),
                        ("Approved on", "20 May 2025 14:32 IST")]))

    st.append(Paragraph("10. Evidence Collected", H1))
    st.append(grid(["Evidence", "Detail", "Reference"],
                   [["Email notifications", "2 emails to existing clients", "Email_Notification_Log.pdf"],
                    ["Agreement version", "v2.0 incorporating MITC", "IA_Agreement_v2.pdf"],
                    ["Acknowledgements", "2 client acknowledgements", "Client_Acknowledgement_Register.pdf"],
                    ["Policy approval", "Internal policy v2.0 approved", "Internal_Compliance_Policy_v2.pdf"],
                    ["Human sign-off", "Compliance Officer approval", "Human_Approval_Record.pdf"]],
                   widths=[36*mm, None, 58*mm]))
    st.append(PageBreak())

    st.append(Paragraph("11. Client Notifications", H1))
    st.append(P("Existing clients were informed of the standardized MITC by email, with delivery and acknowledgement tracked."))
    st.append(grid(["Client", "Notified On", "Channel", "Delivery", "Acknowledged"],
                   [["Rahul Sharma", "22 May 2025", "Email", "Delivered", "28 May 2025"],
                    ["Sneha Iyer", "22 May 2025", "Email", "Delivered", "27 May 2025"]],
                   widths=[36*mm, 28*mm, 24*mm, 24*mm, None]))

    st.append(Paragraph("12. Agreement Updates", H1))
    st.append(P("The client agreement template was updated from v1.0 to v2.0 to incorporate the standardized MITC (Annexure A) and a "
                "client acknowledgement section. The change is recorded in the agreement's revision history."))
    st.append(grid(["Version", "Date", "Change"],
                   [["1.0", "01 Jan 2024", "Baseline agreement (no MITC)"],
                    ["2.0", "20 May 2025", "MITC incorporated; client acknowledgement added"]],
                   widths=[20*mm, 28*mm, None]))

    st.append(Paragraph("13. Policy Updates", H1))
    st.append(P("The Internal Compliance Policy was updated from v1.0 to v2.0 to add the mandatory MITC client-notification workflow, "
                "evidence retention, acknowledgement tracking and updated responsibilities."))
    st.append(PageBreak())

    st.append(Paragraph("14. Outstanding Risks", H1))
    st.append(grid(["Risk", "Severity", "Status", "Mitigation"],
                   [["Client acknowledgement not received in time", "Low", "Closed", "All acknowledgements received before " + DEADLINE],
                    ["Agreement version drift", "Low", "Closed", "Template centrally versioned; v1 superseded"],
                    ["Evidence not linked to obligation", "Low", "Closed", "Clause-to-evidence lineage maintained (Appendix A)"]],
                   widths=[52*mm, 22*mm, 20*mm, None]))

    st.append(Paragraph("15. Final Compliance Status and Attestation", H1))
    st.append(P(f"As at the date of this Pack, {COMPANY} is compliant with SEBI Circular {MITC_REF}. The standardized MITC has been "
                "incorporated into the client engagement, existing clients have been informed and have acknowledged the MITC, the "
                "internal policy has been updated and approved, and a complete, cited evidence trail has been retained."))
    st.append(gap(4))
    st.append(grid(["Assessment", "Result"],
                   [["Regulatory obligation met", "Yes"], ["Human approval obtained", "Yes"],
                    ["Evidence complete and linked", "Yes"], ["Reg 19(3) compliance", "Compliant"]],
                   widths=[80*mm, None]))
    st.append(gap(10))
    st += approval_block([
        ["Prepared by — Compliance Officer", "Priya Menon", "", "05 Jun 2025"],
        ["Reviewed by — Director", "A. Nair", "", "05 Jun 2025"],
        ["Noted by — Board of Directors", "Board", "", "05 Jun 2025"],
    ])
    st.append(PageBreak())

    st.append(Paragraph("Appendix A — Clause-to-Evidence Lineage", H1))
    st.append(grid(["Clause", "Obligation", "Workflow", "Evidence"],
                   [["MITC ¶2", "MITC-1 Provide MITC + acknowledge", "Update agreement; notify clients", "IA_Agreement_v2.pdf; Email_Notification_Log.pdf"],
                    ["MITC ¶3", "3.1 Agreement incorporates MITC", "Update agreement template", "IA_Agreement_v2.pdf"],
                    ["MITC ¶4", "3.4 Retain acknowledgement", "Collect acknowledgements", "Client_Acknowledgement_Register.pdf"],
                    ["MITC ¶5", "5.2 Inform existing clients", "Notify clients by " + DEADLINE, "Email_Notification_Log.pdf"]],
                   widths=[22*mm, None, 42*mm, 52*mm]))
    st.append(gap(10))
    st.append(Paragraph("Appendix B — Document Register", H1))
    st.append(grid(["#", "Document"], [[i+1, d] for i, d in enumerate(DOC_LIST)], widths=[12*mm, None]))
    build("Audit_Pack_Reg19(3).pdf", st, code)

# =============================================================================
# 6 — BLAST RADIUS REPORT
# =============================================================================
def blast_report():
    code = "AWA/BLAST/2025/MITC"
    st = []
    st += cover("Blast Radius Report",
                "Operational Impact of the MITC Amendment",
                [("Regulatory Event", MITC_REF), ("Date", MITC_DATE),
                 ("Prepared by", "Compliance Department"), ("Status", "Compliant after remediation")], code)
    st.append(PageBreak())
    st.append(Paragraph("Propagation of the Regulatory Change", H1))
    flow = ["Regulatory Change (MITC Circular)", "Affected Obligations (3 modified, 1 added)",
            "Affected Policies (Internal Compliance Policy v1 → v2)", "Affected Agreements (Template v1 → v2)",
            "Affected Clients (2 existing clients)", "Evidence Required (5 artefacts)", "Compliance Status (Restored — Compliant)"]
    rows = [[str(i+1), s] for i, s in enumerate(flow)]
    st.append(grid(["Step", "Impact Layer"], rows, widths=[16*mm, None]))
    st.append(gap(8))
    st.append(Paragraph("Impact Summary", H1))
    st.append(grid(["Dimension", "Count", "Detail"],
                   [["Obligations", "4", "3 modified, 1 added"], ["Policies", "1", "Internal Compliance Policy v2"],
                    ["Agreements", "1", "Client agreement template v2"], ["Clients", "2", "Existing clients notified"],
                    ["Evidence", "5", "Email, agreement, acknowledgements, policy, sign-off"], ["Controls", "2", "Existing controls updated"]],
                   widths=[36*mm, 18*mm, None]))
    st.append(gap(8))
    st.append(Paragraph("Semantic Note", H1))
    st.append(P("Beyond the direct structural links, a semantic similarity analysis surfaced the client-notification control as "
                "affected by the fee-and-terms disclosure change — a dependency that a purely structural view would miss. This "
                "confirms the completeness of the impact assessment."))
    st.append(gap(10))
    st += approval_block([["Prepared by — Compliance Officer", "Priya Menon", "", "20 May 2025"]])
    build("Blast_Radius_Report.pdf", st, code)

# =============================================================================
# Short supporting documents (to fully populate the bundle)
# =============================================================================
def compliance_change_report():
    code = "AWA/CCR/2025/MITC"
    st = cover("Compliance Change Report", "MITC Amendment — Summary of Change",
               [("Regulatory Event", MITC_REF), ("Date", MITC_DATE), ("Status", "Implemented")], code)
    st.append(PageBreak())
    st.append(Paragraph("Summary", H1))
    st.append(P(f"SEBI Circular {MITC_REF} dated {MITC_DATE} introduced the standardized Most Important Terms and Conditions (MITC) "
                "for Investment Advisers. The Firm updated its client agreement (v2), internal compliance policy (v2), notified "
                f"existing clients, collected acknowledgements, and retained evidence — completed on or before {DEADLINE}."))
    st.append(grid(["Item", "Before", "After"],
                   [["Agreement", "v1.0 (no MITC)", "v2.0 (MITC incorporated)"],
                    ["Policy", "v1.0", "v2.0 (MITC workflow)"],
                    ["Client acknowledgement", "Not required", "Required and obtained"],
                    ["Compliance status", "Compliant (pre-MITC)", "Compliant (MITC)"]],
                   widths=[46*mm, None, None]))
    build("Compliance_Change_Report.pdf", st, code)

def lineage_report():
    code = "AWA/LINEAGE/2025/MITC"
    st = cover("Clause-to-Evidence Lineage", "Traceability — MITC Amendment",
               [("Regulatory Event", MITC_REF), ("Date", MITC_DATE)], code)
    st.append(PageBreak())
    st.append(Paragraph("Lineage", H1))
    st.append(grid(["Clause", "Obligation", "Workflow", "Evidence", "Sign-off"],
                   [["MITC ¶2", "MITC-1", "Update agreement; notify", "IA_Agreement_v2.pdf", "Priya Menon"],
                    ["MITC ¶3", "3.1", "Update agreement template", "IA_Agreement_v2.pdf", "Priya Menon"],
                    ["MITC ¶4", "3.4", "Collect acknowledgements", "Client_Acknowledgement_Register.pdf", "Priya Menon"],
                    ["MITC ¶5", "5.2", "Notify existing clients", "Email_Notification_Log.pdf", "Priya Menon"]],
                   widths=[20*mm, 18*mm, 40*mm, None, 28*mm]))
    build("Clause_to_Evidence_Lineage.pdf", st, code)

def email_log():
    code = "AWA/EMAIL-LOG/2025/MITC"
    st = cover("Email Notification Log", "MITC Client Notifications",
               [("Regulatory Event", MITC_REF), ("Deadline", DEADLINE)], code)
    st.append(PageBreak())
    st.append(Paragraph("Notifications Sent", H1))
    st.append(grid(["#", "Client", "Email", "Subject", "Sent", "Status"],
                   [["1", "Rahul Sharma", "rahul.sharma@example.in", "Important: Standardized MITC", "22 May 2025 10:14", "Delivered"],
                    ["2", "Sneha Iyer", "sneha.iyer@example.in", "Important: Standardized MITC", "22 May 2025 10:15", "Delivered"]],
                   widths=[8*mm, 30*mm, 48*mm, None, 30*mm, 20*mm]))
    build("Email_Notification_Log.pdf", st, code)

def ack_register():
    code = "AWA/ACK-REG/2025/MITC"
    st = cover("Client Acknowledgement Register", "MITC Acknowledgements",
               [("Regulatory Event", MITC_REF), ("Deadline", DEADLINE)], code)
    st.append(PageBreak())
    st.append(Paragraph("Acknowledgements", H1))
    st.append(grid(["#", "Client", "PAN", "MITC Sent", "Acknowledged", "Mode", "Status"],
                   [["1", "Rahul Sharma", "ABCDE1234F", "22 May 2025", "28 May 2025", "Email + e-Sign", "Complete"],
                    ["2", "Sneha Iyer", "PQRSX6789L", "22 May 2025", "27 May 2025", "Email + e-Sign", "Complete"]],
                   widths=[8*mm, 30*mm, 26*mm, 26*mm, 26*mm, None, 22*mm]))
    build("Client_Acknowledgement_Register.pdf", st, code)

def approval_record():
    code = "AWA/APPROVAL/2025/MITC"
    st = cover("Human Approval Record", "Governance — MITC Amendment",
               [("Regulatory Event", MITC_REF), ("Decision", "Approved")], code)
    st.append(PageBreak())
    st.append(Paragraph("Approval", H1))
    st.append(kv_table([("Reviewer", "Priya Menon — Compliance Officer"),
                        ("Item reviewed", "AI-extracted MITC obligations and operational plan"),
                        ("Decision", "Approved"), ("Confidence", "98%"),
                        ("Citation", MITC_REF), ("Approved on", "20 May 2025 14:32 IST"),
                        ("Note", "No action was enforced before this approval.")]))
    build("Human_Approval_Record.pdf", st, code)

def client_register_csv():
    path = os.path.join(OUT, "Client_Register.csv")
    with open(path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["Client ID", "Name", "PAN", "Category", "Onboarded", "Agreement Version", "MITC Sent", "MITC Acknowledged"])
        w.writerow(["AWA-0001", "Rahul Sharma", "ABCDE1234F", "Individual", "12 Feb 2024", "v2.0", "22 May 2025", "28 May 2025"])
        w.writerow(["AWA-0002", "Sneha Iyer", "PQRSX6789L", "Individual", "03 Sep 2024", "v2.0", "22 May 2025", "27 May 2025"])
    print("wrote", path)

DOC_LIST = [
    "IA_Master_Circular_2025.pdf", "MITC_Circular_17Feb2025.pdf", "IA_Agreement_v1.pdf", "IA_Agreement_v2.pdf",
    "Internal_Compliance_Policy_v1.pdf", "Internal_Compliance_Policy_v2.pdf", "Client_Register.csv",
    "Blast_Radius_Report.pdf", "Compliance_Change_Report.pdf", "Clause_to_Evidence_Lineage.pdf",
    "Audit_Pack_Reg19(3).pdf", "Email_Notification_Log.pdf", "Client_Acknowledgement_Register.pdf",
    "Human_Approval_Record.pdf",
]

# copy the two source circulars into the bundle with the expected names
for src, dst in [(r"C:\Users\mridu\Downloads\IA_Master_Circular_SEBI.pdf", "IA_Master_Circular_2025.pdf"),
                 (r"C:\Users\mridu\Downloads\SEBI MITC Circular (17 Feb 2025).pdf", "MITC_Circular_17Feb2025.pdf")]:
    try:
        shutil.copyfile(src, os.path.join(OUT, dst)); print("copied", dst)
    except Exception as e:
        print("WARN copy", dst, e)

agreement(1, "01 January 2024", with_mitc=False)
agreement(2, "20 May 2025", with_mitc=True)
compliance_policy(1, "01 April 2023", updated=False)
compliance_policy(2, "20 May 2025", updated=True)
audit_pack()
blast_report()
compliance_change_report()
lineage_report()
email_log()
ack_register()
approval_record()
client_register_csv()
print("DONE")
