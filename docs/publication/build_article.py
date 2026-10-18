"""Build the three-page article and editable architecture diagram from local sources.

Requires reportlab, Pillow, pypdf and pypdfium2. No model inference or network calls.
"""
import html
import re
from io import BytesIO
from pathlib import Path

import pypdfium2 as pdfium
from pypdf import PdfReader
from reportlab.graphics import renderPDF, renderSVG
from reportlab.graphics.shapes import Drawing, Line, Polygon, Rect, String
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.platypus import Image, PageBreak, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parent
ASSETS = ROOT / 'assets'
NAVY = colors.HexColor('#09232d')
LIME = colors.HexColor('#c8f275')
GRAY = colors.HexColor('#5a686c')


def architecture():
    d = Drawing(1600, 760)
    d.add(Rect(0, 0, 1600, 760, fillColor=colors.HexColor('#fafbf6'), strokeColor=None))

    def text(x, y, content, size=25, bold=False, fill=NAVY):
        d.add(String(x, 760-y, content, fontName='Helvetica-Bold' if bold else 'Helvetica', fontSize=size, fillColor=fill))

    def box(x, y, w, h, title, subtitle='', fill=colors.white):
        d.add(Rect(x, 760-y-h, w, h, rx=14, ry=14, fillColor=fill, strokeColor=NAVY, strokeWidth=1.8))
        text(x+22, y+35, title, 26, True)
        if subtitle:
            text(x+22, y+65, subtitle, 20, fill=GRAY)

    def arrow(x, y, xx, yy, dashed=False):
        line = Line(x, 760-y, xx, 760-yy, strokeColor=NAVY, strokeWidth=2.5)
        if dashed:
            line.strokeDashArray = [7, 5]
        d.add(line)
        if xx == x:
            sign = 1 if yy > y else -1
            points = [xx, 760-yy, xx-7, 760-yy+sign*12, xx+7, 760-yy+sign*12]
        else:
            sign = 1 if xx > x else -1
            points = [xx, 760-yy, xx-sign*12, 760-yy-7, xx-sign*12, 760-yy+7]
        d.add(Polygon(points, fillColor=NAVY, strokeColor=None))

    text(30, 40, 'PANLYACKERP | APPLICATION ARCHITECTURE', 30, True)
    box(30, 65, 1540, 80, 'Browser: public site, sign-up / sign-in, bilingual workspace', 'Invoices, JSON export, stock, ledger, cash and dashboard | templ + HTMX + Tailwind')
    arrow(545, 145, 545, 185)
    box(30, 185, 1030, 365, 'Go + Chi | modular monolith', 'Session-derived organization scope, CSRF, exact decimal validation')
    box(58, 275, 974, 65, 'Workflows: registration, invoice posting, settlement', fill=LIME)
    arrow(545, 340, 545, 368)
    for x, y, title in [(58, 370, 'Identity'), (388, 370, 'Documents'), (718, 370, 'Inventory'),
                        (58, 455, 'Accounting'), (388, 455, 'Cash'), (718, 455, 'Reporting')]:
        box(x, y, 314, 65, title)
    box(1110, 185, 460, 365, 'FastAPI | bounded HTTP')
    text(1132, 282, 'EasyOCR + Gemma 3 4B', 24, True)
    text(1132, 316, 'Image / PDF -> draft fields', 21)
    text(1132, 375, 'Optional US role advisor', 24, True)
    text(1132, 410, 'SmolVLM LoRA + local retrieval', 21)
    text(1132, 474, 'Read-only suggestions', 23, True)
    text(1132, 508, 'No ERP database access', 21)
    arrow(1060, 245, 1110, 245, dashed=True)
    arrow(545, 550, 545, 610)
    box(30, 610, 1030, 100, 'PostgreSQL | module-owned schemas', 'Atomic posting: stock + journal + cash + audit + outbox + projection', fill=LIME)
    box(1110, 610, 460, 100, 'Local document files', 'Bounded uploads + SHA-256')
    arrow(545, 575, 1340, 575, dashed=True)
    arrow(1340, 575, 1340, 610, dashed=True)
    text(30, 744, 'Modules do not import each other. Workflows coordinate ports. Dashed lines denote service or file boundaries.', 20)
    (ASSETS / 'application-architecture.svg').write_text(renderSVG.drawToString(d), encoding='utf-8')
    raw = renderPDF.drawToString(d)
    doc = pdfium.PdfDocument(raw)
    doc[0].render(scale=2).to_pil().save(ASSETS / 'application-architecture.png')
    doc.close()


def inline(text):
    text = html.escape(text)
    text = re.sub(r'\[([^\]]+)\]\(([^)]+)\)', lambda m: f'<link href="{m[2]}" color="#225765">{m[1]}</link>', text)
    text = re.sub(r'\*\*(.+?)\*\*', r'<b>\1</b>', text)
    text = re.sub(r'\*([^*]+)\*', r'<i>\1</i>', text)
    return text


def article():
    styles = getSampleStyleSheet()
    styles.add(ParagraphStyle(name='BodyArticle', fontName='Times-Roman', fontSize=10.2, leading=13.1, textColor=NAVY, spaceAfter=7, alignment=TA_LEFT))
    styles.add(ParagraphStyle(name='ArticleTitle', fontName='Helvetica-Bold', fontSize=20, leading=23, textColor=NAVY, spaceAfter=9))
    styles.add(ParagraphStyle(name='Section', fontName='Helvetica-Bold', fontSize=11.2, leading=14.2, textColor=NAVY, spaceBefore=8, spaceAfter=5, keepWithNext=True))
    styles.add(ParagraphStyle(name='CaptionArticle', fontName='Times-Italic', fontSize=8.6, leading=10.8, textColor=GRAY, spaceAfter=8))
    styles.add(ParagraphStyle(name='CellArticle', fontName='Times-Roman', fontSize=8.6, leading=10.8, textColor=NAVY))
    styles.add(ParagraphStyle(name='ReferenceArticle', fontName='Times-Roman', fontSize=8.1, leading=10.1, textColor=NAVY, spaceAfter=3))
    width = A4[0] - 84
    story = []
    blocks = re.split(r'\n\s*\n', (ROOT / 'article.md').read_text(encoding='utf-8').strip())
    references = False
    for block in blocks:
        block = block.strip()
        if block == '<!-- page -->':
            story.append(PageBreak())
        elif block.startswith('# '):
            story.append(Paragraph(inline(block[2:]), styles['ArticleTitle']))
        elif block.startswith('## '):
            references = block == '## References'
            story.append(Paragraph(inline(block[3:]), styles['Section']))
        elif block.startswith('!['):
            path = re.search(r'\]\((.*?)\)', block)[1]
            image = Image(str(ROOT / path))
            image.drawHeight *= width / image.drawWidth
            image.drawWidth = width
            story.extend([Spacer(1, 5), image, Spacer(1, 5)])
        elif block.startswith('|'):
            rows = [row.strip().strip('|').split('|') for row in block.splitlines() if not re.match(r'^\|\s*---', row)]
            data = [[Paragraph(inline(cell.strip()), styles['CellArticle']) for cell in row] for row in rows]
            table = Table(data, colWidths=[width*.245, width*.425, width*.33], repeatRows=1)
            table.setStyle(TableStyle([('BACKGROUND', (0,0), (-1,0), LIME), ('VALIGN',(0,0),(-1,-1),'TOP'), ('BOX',(0,0),(-1,-1),.4,colors.HexColor('#cad3c5')), ('INNERGRID',(0,0),(-1,-1),.3,colors.HexColor('#dce1d6')), ('TOPPADDING',(0,0),(-1,-1),6), ('BOTTOMPADDING',(0,0),(-1,-1),6)]))
            story.extend([table, Spacer(1, 8)])
        else:
            style = styles['CaptionArticle'] if block.startswith('**Figure ') else styles['ReferenceArticle'] if references else styles['BodyArticle']
            if references:
                for line in block.splitlines():
                    story.append(Paragraph(inline(line), style))
            else:
                story.append(Paragraph(inline(block.replace('\n', ' ')), style))

    def decorate(canvas, doc):
        canvas.saveState()
        canvas.setFillColor(NAVY)
        canvas.rect(42, A4[1]-27, 20, 3, fill=1, stroke=0)
        canvas.setFont('Helvetica', 8)
        canvas.drawString(70, A4[1]-28, 'PANLYACKERP / TECHNICAL PROJECT REPORT')
        canvas.setFillColor(GRAY)
        canvas.drawString(42, 24, 'Joy | 18 October 2026 | github.com/joeylukoji/PanlyackErp')
        canvas.drawRightString(A4[0]-42, 24, f'{doc.page} / 3')
        canvas.restoreState()

    output = ROOT / 'PanlyackERP-Technical-Article.pdf'
    doc = SimpleDocTemplate(str(output), pagesize=A4, leftMargin=42, rightMargin=42, topMargin=42, bottomMargin=42, title='PanlyackERP: From Invoice Evidence to Consistent Business Records', author='Joy', subject='Technical implementation and bounded evaluation of an invoice-centered ERP')
    doc.build(story, onFirstPage=decorate, onLaterPages=decorate)
    reader = PdfReader(output)
    assert len(reader.pages) == 3, f'Expected exactly 3 pages, got {len(reader.pages)}'
    full_text = '\n'.join(page.extract_text() for page in reader.pages)
    assert '0.362865' in full_text and '46/46' in full_text
    assert not re.search(r'\b(?:OpenAI|ChatGPT|artificial intelligence)\b', full_text, re.I)
    preview = ROOT.parents[1] / 'var/publication-review'
    preview.mkdir(parents=True, exist_ok=True)
    pdf = pdfium.PdfDocument(str(output))
    for i in range(len(pdf)):
        pdf[i].render(scale=1.65).to_pil().save(preview / f'article-page-{i+1}.png')
    pdf.close()
    print(f'Created {output}: 3 pages')


if __name__ == '__main__':
    architecture()
    article()
