# Expanded invoice pack with new test documents

The update inspects the authoritative full-image annotation collections under `C:/Users/Joy/Downloads/facture_datasets`: 108 local receipts in `outputs/photos_factures` and 85 supplier invoices in `facture_fournisseurs`. The 772 PNG files are labelled field crops, not full invoice documents. Root-level legacy examples and duplicate supplier previews are excluded.

Comparing image bytes alone is insufficient: all 108 current receipt JPEG files differ from their earlier copies, but 50 describe invoices already represented in the frozen pilot. The selection compares both image SHA-256 and normalized invoice number/supplier/printed-date identity. Total is deliberately excluded from identity so changing an amount label cannot turn a known invoice into an independent test case.

Discovery found 35 byte-identical known supplier invoices and 50 re-rendered known receipts. These 85 source records are logged as exclusions. The remaining 108 previously unused documents comprise 58 receipts and 50 supplier invoices. The 58 include eight older receipts that were not selected for the original pilot, plus 50 newly added receipts. They are unseen by the pilot; they are not all newly created files.

The self-contained expanded pack is `D:/PanlyackERP-training/dataset-expanded`:

| Partition | Images | Use |
| --- | ---: | --- |
| Training | 142 | Preserved 77 original parents plus 65 mild variants |
| Validation | 8 | The same eight original held-out images |
| Test | 108 | Previously unused source documents, copied unchanged |
| Total | 258 | 193 distinct invoice/receipt documents plus 65 variants |

The original `dataset-150` remains intact. All new test images are originals; none has an augmented training copy. All 432 labelled crops of those 108 test documents stay with their test parents. OCR reads the full image and uses automatic header/footer regions; it never uses the labelled crop boxes or expected field values as an oracle.

The pack contains per-document JSON targets/full supplied annotations, copied labelled crops, original CSV/JSON/crop-index source snapshots and hashes, OCR predictions, a discovery ledger and verification report. The gallery at `http://127.0.0.1:8096/` shows the new test cases; its previous-gallery link retains the 150-image view. Source labels remain synthetic and unconfirmed; matching CSV/JSON does not prove agreement with visible content.

Per-document supplied decimal values are serialized as strings to preserve numeric precision. The source snapshots retain the original JSON/CSV bytes and types, including numeric box coordinates. OCR confidence and geometry are predictions in their own processed-image coordinate space, separate from the supplied positional labels.

```powershell
# One-time expansion; refuses to overwrite an existing output folder.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/extend-ocr-dataset.py --gpu

# Resume OCR without modifying the frozen source selection.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/extend-ocr-dataset.py --ocr-only --gpu

# A future training run uses only train and validation, never test.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/train-light-invoices.py --dataset D:/PanlyackERP-training/dataset-expanded --output D:/PanlyackERP-training/invoice-ocr-next

# Explicit final-test evaluation, only after choosing a checkpoint using validation.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/evaluate-light-invoices.py --run D:/PanlyackERP-training/invoice-ocr-next --dataset D:/PanlyackERP-training/dataset-expanded --split test
```

The loader rejects shared parent hashes and fully specified invoice identities across train, validation and test. It returns only training/validation records to the trainer and never loads test OCR into training context. Test generation writes `test-generation-evaluation.json` separately. The update does not retrain or evaluate a model on these 108 documents; they remain reserved for final testing. Supplier/layout independence is still unproven because the generators reuse templates.

Completed checks: 258 images decoded and hashed, 258 matching nonempty OCR caches, 432 decoded field crops with matching hashes, preserved parent dimensions/targets and zero parent/identity overlap across all three partitions. Source CSV and JSON header fields agreed for all 193 input records. Gallery, final image, label, crop, OCR and verification links returned HTTP 200. The first/last selected test images were visually inspected; this is not human confirmation of the whole test set. Python suite: 45 tests passed with the existing Starlette/httpx deprecation warning.

Subsequently, the image-plus-OCR run at `D:/PanlyackERP-training/invoice-smolvlm-ocr-lora` completed training on the 142 training images and generation evaluation on the eight validation images. Its results are recorded in [OCR support and augmentation](ocr-and-augmentation.md). The 108 test documents remain reserved and unevaluated. The pack manifest's `training_started=false` describes its preparation state, not the status of this later, separately recorded run.
