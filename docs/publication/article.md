# PanlyackERP: From Invoice Evidence to Consistent Business Records

**Joy** | Technical project report | 18 October 2026

## Abstract

A small business may record the same purchase in an invoice folder, stock sheet, cashbook and ledger. Re-entry creates inconsistent quantities, payments and balances. I developed PanlyackERP around a single reviewed document: extraction proposes data, a person confirms it, and a deterministic transaction updates the business records together. The implementation combines local document recognition, lightweight adapter experiments and a modular ERP. This report describes the implemented system and its measured limitations; it does not claim general invoice accuracy or certified accounting compliance.

## 1. Problem and design objective

Recognizing text is only the first step. A supplier name can be mistaken for the recipient; a quantity can be confused with a price; an unpaid invoice can be mistaken for payment evidence. Even correctly extracted fields do not establish business purpose, tax treatment or the right stock movement. The engineering objective is therefore to preserve a traceable path from uncertain document evidence to explicitly approved, internally consistent records.

Two inputs require different treatment. External supplier invoices and expense documents accept an uploaded image, PDF or browser camera input. Sales authored inside the application already contain structured fields: they produce a printable invoice and downloadable JSON without recognition. Both paths retain confirmation before posting.

## 2. Implementation and consistency boundary

I implemented a Go modular monolith with Chi, templ, HTMX and Tailwind, PostgreSQL persistence, and a separate FastAPI recognition service. Server-rendered pages keep business state on the server and support French and English. Identity, documents, inventory, accounting, cash and reporting remain separate modules; workflows coordinate their ports. Recognition has no ERP database access.

![Figure 1. Application architecture.](assets/application-architecture.png)

**Figure 1.** Session-scoped commands enter Go workflows. One PostgreSQL transaction commits posting effects, audit, a persisted outbox record and the synchronous dashboard projection. The outbox has no asynchronous dispatcher yet.

Money is represented as decimal strings at boundaries and exact decimals in Go. Posting locks the invoice, requires its confirmed state and commits stock, balanced entries and payments atomically. Retrying an already posted invoice produces no duplicate effects. Later settlements have their own idempotency key. Posted records cannot be edited through the application; a persisted reversal interface remains future work.

<!-- page -->

## 3. Recognition and local adaptation

EasyOCR supplies bounded French/English text and spatial evidence from five automatic overlapping views [1]. Images are oriented and normalized; PDFs are rasterized within page and size limits. Gemma 3 4B, served locally through Ollama, reads original pages plus detail evidence and proposes schema-constrained headers and product rows [2]. Decimal diagnostics compare lines, taxes, totals and explicit payments. At most one corrective reading is allowed; unresolved discrepancies remain visible for review. Gemma is not fine-tuned in this project.

![Figure 2. Runtime and adapter pipeline.](assets/invoice-model-pipeline.png)

**Figure 2.** The live recognition path and the two SmolVLM experiments are distinct. The invoice-header adapter is not used by the live extractor. Accounting suggestions are read-only and checked against deterministic roles.

I used SmolVLM-256M-Instruct [3] for two rank-eight LoRA adaptations [4], updating query/value attention projections while freezing the base weights. The invoice experiment trains only number, supplier, printed date and total, not product rows. It uses BF16, batch size one, accumulation four, learning rate 1e-4 and three epochs: 142 training images comprise 77 original parents and 65 mild photometric variants. Eight original images form validation; 108 additional documents remain reserved. Hash and invoice-identity checks keep parents and variants together across partitions. Labels are supplied synthetic targets, not human-confirmed ground truth.

The accounting experiment uses 184 synthetic training cases, 46 validation cases and 46 reserved tests, with separate product descriptions. Four epochs combine target-only loss with training-only retrieval. Retrieval also uses six reference cards and selected public accounting vocabulary; at most four sources enter a prompt. It uses operation filters, structural matching and lexical overlap, without a vector database. Public expenditure descriptions provide terminology, not corporate journal supervision.

<!-- page -->

## 4. Evaluation and end-to-end evidence

Experiments ran on a Ryzen 7 8845HS laptop with approximately 16 GB RAM and an RTX 4060 Laptop GPU with 8 GB VRAM. Training and full-stack inference are not run simultaneously. Model hashes, split checks and completed reports distinguish a saved experiment from a prepared dataset.

| Experiment | Observed validation result | Interpretation |
| --- | --- | --- |
| Image-only header adapter | Parseable JSON 5/8; number 0/8; supplier 2/8; date 4/8; total 1/8 | Insufficient extraction quality |
| Image + OCR header adapter | JSON 8/8; number 3/8; supplier 5/8; date 8/8; total 5/8 exact, 6/8 decimal-normalized | Small synthetic pilot only |
| US account-role adapter + retrieval | Schema-valid and exact role matches 46/46 | Structural synthetic validation, not independent accounting accuracy |

The OCR-assisted run reduced teacher-forced validation loss from 1.584853 to 0.362865 and peaked at 5.764 GiB allocated tensor memory. Its adapter weighs 1,860,096 bytes, excluding base weights and processor files. The accounting run peaked at 2.654 GiB. These results do not isolate the benefit of OCR or fine-tuning: augmentation, exposure and retrieval differ between setups. Reused layouts, cropped supplier headings and repeated accounting structures restrict generalization. Both final test partitions remain unevaluated; product-line recognition has no independent accuracy estimate.

A fictional end-to-end demonstration purchases ten keyboards for USD 250, pays 150 and retains a 100 payable. A sale of two units at 40 plus 8% entered sales tax totals 86.40; cash 50 and a later bank receipt 36.40 settle it. Final records show eight units, inventory 200, revenue 80, cost of goods sold 50, cash 4,900 and bank 36.40. The sale exports structured JSON directly. This verifies connected workflow behavior, not recognition accuracy or tax applicability.

## 5. Scope, reproducibility and next steps

The bounded US policy capitalizes nonrecoverable purchase tax and separates buyer-imposed sales tax from revenue. The operator establishes purpose, jurisdiction, rate and payment evidence. Tax filing, nexus, exemptions, bank reconciliation and certified compliance are outside scope. A printed-cent rounding discrepancy of 0.002 remains an explicit policy gap. The application is a loopback-bound MVP with production mode disabled, not a deployed multi-user service.

The repository includes source, migrations, tests, training scripts, report excerpts and reproducibility instructions. Local invoices, credentials and model weights are excluded. Next steps are human-confirmed visible-field references, unseen supplier/layout evaluation, an independently reviewed accounting benchmark, rounding and reversal workflows, and production identity hardening. The contribution is an auditable separation between document suggestions and deterministic business effects.

## References

1. JaidedAI. EasyOCR, version 1.7.2. https://github.com/JaidedAI/EasyOCR
2. Gemma Team. *Gemma 3 Technical Report*, 2025. https://arxiv.org/abs/2503.19786
3. Marafioti et al. *SmolVLM: Redefining small and efficient multimodal models*, 2025. https://arxiv.org/abs/2504.05299
4. Hu et al. *LoRA: Low-Rank Adaptation of Large Language Models*, 2021. https://arxiv.org/abs/2106.09685
5. Implementation, reports and reproduction: https://github.com/joeylukoji/PanlyackErp/tree/main/docs/publication
