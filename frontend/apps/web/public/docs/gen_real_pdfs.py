from fpdf import FPDF
import datetime
import os

OUT_DIR = os.path.join(".", "frontend", "apps", "web", "public", "docs")
os.makedirs(OUT_DIR, exist_ok=True)

FIRM = "Acme Investment Advisers"
CIN = "U74999MH2021PTC351234"
ADDRESS = "12th Floor, Tower B, Peninsula Business Park, Lower Parel, Mumbai - 400013"

class GovtPDF(FPDF):
    def header(self):
        if hasattr(self, 'doc_type') and self.doc_type == "SEBI":
            self.set_font('Helvetica', 'B', 24)
            self.cell(0, 15, 'SECURITIES AND EXCHANGE BOARD OF INDIA', ln=True, align='C')
            self.set_font('Helvetica', 'B', 14)
            self.cell(0, 10, '(INVESTMENT ADVISERS) REGULATIONS, 2013', ln=True, align='C')
            self.ln(10)
            self.set_line_width(0.5)
            self.line(20, self.get_y(), 190, self.get_y())
            self.ln(10)
        elif hasattr(self, 'doc_type') and self.doc_type == "GST":
            self.set_font('Helvetica', 'B', 20)
            self.cell(0, 15, 'GOVERNMENT OF INDIA', ln=True, align='C')
            self.set_font('Helvetica', 'B', 14)
            self.cell(0, 10, 'FORM GST REG-06', ln=True, align='C')
            self.set_font('Helvetica', '', 12)
            self.cell(0, 8, '[See Rule 10(1)]', ln=True, align='C')
            self.ln(5)
            self.set_font('Helvetica', 'B', 16)
            self.cell(0, 10, 'Registration Certificate', ln=True, align='C')
            self.ln(5)
            self.set_line_width(0.5)
            self.line(20, self.get_y(), 190, self.get_y())
            self.ln(10)
        elif hasattr(self, 'doc_type') and self.doc_type == "CORP":
            self.set_font('Helvetica', 'B', 24)
            self.cell(0, 15, FIRM.upper(), ln=True, align='L')
            self.set_font('Helvetica', '', 10)
            self.cell(0, 5, f'CIN: {CIN}', ln=True, align='L')
            self.cell(0, 5, ADDRESS, ln=True, align='L')
            self.ln(5)
            self.set_line_width(0.5)
            self.line(10, self.get_y(), 200, self.get_y())
            self.ln(10)

def generate_sebi():
    pdf = GovtPDF()
    pdf.doc_type = "SEBI"
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 18)
    pdf.cell(0, 15, 'CERTIFICATE OF REGISTRATION', ln=True, align='C')
    pdf.ln(10)
    
    pdf.set_font('Helvetica', '', 12)
    text1 = "In exercise of the powers conferred by sub-section (1) of section 12 of the Securities and Exchange Board of India Act, 1992, read with the rules and regulations made thereunder, the Board hereby grants a certificate of registration to:"
    pdf.multi_cell(0, 8, text1, align='J')
    pdf.ln(5)
    
    pdf.set_font('Helvetica', 'B', 16)
    pdf.cell(0, 12, FIRM, ln=True, align='C')
    pdf.set_font('Helvetica', '', 12)
    pdf.multi_cell(0, 8, ADDRESS, align='C')
    pdf.ln(10)
    
    text2 = "as an Investment Adviser subject to the conditions specified in the Act and in the rules and regulations made thereunder."
    pdf.multi_cell(0, 8, text2, align='J')
    pdf.ln(10)
    
    pdf.set_font('Helvetica', 'B', 14)
    pdf.cell(0, 10, 'Registration Number: INA000012345', ln=True, align='C')
    pdf.ln(30)
    
    pdf.set_font('Helvetica', '', 12)
    pdf.cell(95, 8, 'Date: January 15, 2024', ln=False, align='L')
    pdf.cell(95, 8, 'By Order', ln=True, align='R')
    pdf.ln(15)
    pdf.cell(95, 8, 'Place: Mumbai', ln=False, align='L')
    pdf.cell(95, 8, 'For and on behalf of', ln=True, align='R')
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(190, 8, 'Securities and Exchange Board of India', ln=True, align='R')
    
    pdf.output(os.path.join(OUT_DIR, "sebi_registration.pdf"))

def generate_gst():
    pdf = GovtPDF()
    pdf.doc_type = "GST"
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 12)
    
    data = [
        ("1. Legal Name", FIRM),
        ("2. Trade Name, if any", FIRM),
        ("3. Constitution of Business", "Private Limited Company"),
        ("4. Address of Principal Place of Business", ADDRESS),
        ("5. Date of Liability", "12/05/2023"),
        ("6. Period of Validity", "From 12/05/2023 To NA"),
        ("7. Type of Registration", "Regular"),
        ("8. Particulars of Approving Authority", "Maharashtra State GST Department")
    ]
    
    for label, val in data:
        pdf.set_font('Helvetica', 'B', 11)
        pdf.cell(70, 10, label, border=1)
        pdf.set_font('Helvetica', '', 11)
        pdf.multi_cell(120, 10, val, border=1, align='L')
        
    pdf.ln(20)
    pdf.set_font('Helvetica', '', 12)
    pdf.cell(95, 8, 'Date of issue: 12/05/2023', ln=False, align='L')
    pdf.cell(95, 8, 'Signature: ______________', ln=True, align='R')
    pdf.ln(5)
    pdf.cell(95, 8, 'Place: Mumbai', ln=False, align='L')
    pdf.cell(95, 8, 'Designation: State Tax Officer', ln=True, align='R')

    pdf.output(os.path.join(OUT_DIR, "gst_certificate.pdf"))

def generate_dpa():
    pdf = GovtPDF()
    pdf.doc_type = "CORP"
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 16)
    pdf.cell(0, 15, 'DATA PROCESSING ADDENDUM (DPA)', ln=True, align='C')
    pdf.ln(5)
    
    pdf.set_font('Helvetica', '', 12)
    pdf.multi_cell(0, 6, "This Data Processing Addendum (\"DPA\") is entered into by and between Acme Investment Advisers (\"Data Fiduciary\") and the Data Processor, in compliance with the Digital Personal Data Protection (DPDP) Act, 2023.")
    pdf.ln(10)
    
    sections = [
        ("1. Definitions", "For the purposes of this DPA, 'Personal Data', 'Data Fiduciary', 'Data Processor', and 'Data Principal' shall have the meanings ascribed to them under the DPDP Act, 2023."),
        ("2. Scope and Purpose of Processing", "The Data Processor shall process Personal Data solely on behalf of Acme Investment Advisers and strictly in accordance with the documented instructions provided herein."),
        ("3. Security Measures", "The Data Processor shall implement appropriate technical and organizational measures (including AES-256 encryption) to ensure a level of security appropriate to the risk, specifically to prevent personal data breaches."),
        ("4. Data Residency", "All processing of Personal Data under this agreement shall occur strictly within the territory of India (ap-south-1) to comply with localized data residency mandates."),
        ("5. Data Principal Rights", "The Data Processor shall assist the Data Fiduciary by appropriate technical and organizational measures for the fulfillment of the Data Fiduciary's obligation to respond to requests for exercising the Data Principal's rights."),
    ]
    
    for title, content in sections:
        pdf.set_font('Helvetica', 'B', 12)
        pdf.cell(0, 10, title, ln=True)
        pdf.set_font('Helvetica', '', 11)
        pdf.multi_cell(0, 6, content)
        pdf.ln(5)
        
    pdf.ln(10)
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(95, 8, 'For Data Fiduciary', ln=False, align='L')
    pdf.cell(95, 8, 'For Data Processor', ln=True, align='L')
    pdf.set_font('Helvetica', '', 12)
    pdf.cell(95, 8, 'Name: Priya Menon (Compliance Officer)', ln=False, align='L')
    pdf.cell(95, 8, 'Name: ______________________', ln=True, align='L')

    pdf.output(os.path.join(OUT_DIR, "dpa.pdf"))

def generate_board_res():
    pdf = GovtPDF()
    pdf.doc_type = "CORP"
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 14)
    pdf.multi_cell(0, 8, f'CERTIFIED TRUE COPY OF THE RESOLUTION PASSED AT THE MEETING OF THE BOARD OF DIRECTORS OF {FIRM.upper()} HELD ON 05 APRIL 2026', align='C')
    pdf.ln(10)
    
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(0, 8, 'APPROVAL OF COMPLIANCE AUTOMATION SYSTEM (CHANAKYA)', ln=True)
    pdf.set_font('Helvetica', '', 12)
    text = f"""
"RESOLVED THAT the Board of Directors of {FIRM} (the 'Company') do hereby approve the adoption and implementation of the 'Chanakya' Compliance Automation System to manage regulatory obligations, enterprise mapping, and audit trails as required by the Securities and Exchange Board of India (SEBI).

RESOLVED FURTHER THAT Ms. Priya Menon, Compliance Officer of the Company, be and is hereby authorized to execute all such documents, agreements, and data processing addendums as may be necessary for the deployment of said system.

RESOLVED FURTHER THAT the Company ensures all client data ingested into the system remains strictly within data centers located in Mumbai (ap-south-1), in accordance with localization mandates.

CERTIFIED TO BE TRUE,
For {FIRM}
"""
    pdf.multi_cell(0, 7, text)
    pdf.ln(20)
    pdf.cell(0, 8, '_________________________', ln=True)
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(0, 8, 'Managing Director', ln=True)
    
    pdf.output(os.path.join(OUT_DIR, "board_resolution.pdf"))

def generate_soc2():
    pdf = GovtPDF()
    pdf.doc_type = "CORP"
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 18)
    pdf.cell(0, 15, 'INFORMATION SECURITY POLICY', ln=True, align='C')
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(0, 10, 'SOC 2 Type II Validated Guidelines', ln=True, align='C')
    pdf.ln(10)
    
    pdf.set_font('Helvetica', '', 11)
    text = f"This Information Security Policy governs the data protection, access controls, and network security standards for all operations at {FIRM}."
    pdf.multi_cell(0, 7, text)
    pdf.ln(5)
    
    sections = [
        "1. Access Control: Role-based access control (RBAC) is strictly enforced. All enterprise systems require MFA.",
        "2. Encryption: All data is encrypted at rest using AES-256 and in transit using TLS 1.3.",
        "3. Audit Logging: Continuous logging is enabled across all infrastructure. Logs are immutable and retained for 5 years.",
        "4. Vendor Risk Management: All third-party integrations (OAuth scopes) are evaluated quarterly for least-privilege compliance.",
        "5. Incident Response: In the event of a security anomaly, the Incident Response Team (IRT) must be notified within 15 minutes."
    ]
    
    for sec in sections:
        pdf.multi_cell(0, 7, sec)
        pdf.ln(2)
        
    pdf.ln(20)
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(0, 8, 'Document Classification: STRICTLY CONFIDENTIAL', ln=True, align='C')
    pdf.output(os.path.join(OUT_DIR, "info_security_policy.pdf"))

def generate_audit():
    pdf = GovtPDF()
    pdf.add_page()
    
    pdf.set_font('Helvetica', 'B', 28)
    pdf.ln(40)
    pdf.cell(0, 15, 'ANNUAL COMPLIANCE', ln=True, align='R')
    pdf.cell(0, 15, 'AUDIT REPORT', ln=True, align='R')
    pdf.ln(10)
    
    pdf.set_font('Helvetica', '', 16)
    pdf.cell(0, 10, 'Financial Year 2025-2026', ln=True, align='R')
    pdf.ln(40)
    
    pdf.set_font('Helvetica', 'B', 14)
    pdf.cell(0, 10, f'Prepared for: {FIRM}', ln=True, align='R')
    pdf.set_font('Helvetica', '', 12)
    pdf.cell(0, 8, f'CIN: {CIN}', ln=True, align='R')
    pdf.ln(60)
    
    pdf.set_line_width(1)
    pdf.line(20, pdf.get_y(), 190, pdf.get_y())
    pdf.ln(10)
    
    pdf.set_font('Helvetica', 'B', 14)
    pdf.cell(0, 10, 'Executive Summary', ln=True)
    pdf.set_font('Helvetica', '', 11)
    summary = f"We have conducted a comprehensive audit of the regulatory and compliance posture of {FIRM} as of December 20, 2025. Based on our evaluation of internal controls, enterprise gap analysis (powered by Chanakya), and obligation mapping, the firm demonstrates a robust compliance framework in accordance with SEBI guidelines."
    pdf.multi_cell(0, 7, summary)
    
    pdf.ln(40)
    pdf.set_font('Helvetica', 'B', 12)
    pdf.cell(0, 8, 'Auditor: Deloitte Touche Tohmatsu India LLP', ln=True, align='R')
    
    pdf.output(os.path.join(OUT_DIR, "annual_audit.pdf"))

if __name__ == "__main__":
    generate_sebi()
    generate_gst()
    generate_dpa()
    generate_board_res()
    generate_soc2()
    generate_audit()
    print("All precise PDFs generated successfully.")
