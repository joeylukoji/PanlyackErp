"""Replace the supplied report's figures without rewriting its text or page layout.

Requires pypdf, Pillow and pypdfium2. PNG previews are rendered from the supplied
SVGs with their embedded fonts; the SVG files themselves are preserved.
"""
import argparse
from pathlib import Path

import pypdfium2 as pdfium
from PIL import Image, ImageOps
from pypdf import PdfReader, PdfWriter

ROOT = Path(__file__).resolve().parent
OUTPUT = ROOT / 'PanlyackERP-Technical-Article.pdf'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    args = parser.parse_args()
    reader = PdfReader(args.source)
    before = [p.extract_text() for p in reader.pages]
    writer = PdfWriter(clone_from=reader)
    replacements = [(1, 'application-architecture.png'), (2, 'invoice-model-pipeline.png')]
    for page_index, filename in replacements:
        page = writer.pages[page_index]
        assert len(page.images) == 1, 'Expected one figure on this source page'
        original = page.images[0]
        # Keep the original image rectangle; letterbox instead of stretching.
        size = tuple(n * 2 for n in original.image.size)
        canvas = Image.new('RGB', size, 'white')
        with Image.open(ROOT / 'assets' / filename) as source:
            image = ImageOps.contain(source.convert('RGB'), size, Image.Resampling.LANCZOS)
        canvas.paste(image, ((size[0]-image.width)//2, (size[1]-image.height)//2))
        original.replace(canvas)
    with OUTPUT.open('wb') as stream:
        writer.write(stream)
    result = PdfReader(OUTPUT)
    assert len(result.pages) == 6
    assert [p.extract_text() for p in result.pages] == before, 'Text changed during figure replacement'
    preview = ROOT.parents[1] / 'var/publication-review/updated-report'
    preview.mkdir(parents=True, exist_ok=True)
    doc = pdfium.PdfDocument(str(OUTPUT))
    for index in range(len(doc)):
        doc[index].render(scale=1.3).to_pil().save(preview / f'page-{index+1}.png')
    doc.close()
    print('Updated both figures; all six pages and source text preserved.')


if __name__ == '__main__':
    main()
