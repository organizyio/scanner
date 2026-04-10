use crate::types::{DocxMeta, PdfMeta};
use roxmltree::Document;
use std::fs::File;
use std::io::Read;
use std::path::Path;

pub fn read_pdf_info(path: &Path) -> Option<PdfMeta> {
    let doc = lopdf::Document::load(path).ok()?;
    let n = doc.get_pages().len();
    if n == 0 {
        return None;
    }
    Some(PdfMeta {
        page_count: Some(n as u32),
        author: None,
        title: None,
    })
}

/// Read `docProps/core.xml` from a DOCX (ZIP) package.
pub fn read_docx_core(path: &Path) -> Option<DocxMeta> {
    let file = File::open(path).ok()?;
    let mut archive = zip::ZipArchive::new(file).ok()?;
    let mut zf = archive.by_name("docProps/core.xml").ok()?;
    let mut xml = String::new();
    zf.read_to_string(&mut xml).ok()?;
    let doc = Document::parse(&xml).ok()?;

    let mut out = DocxMeta::default();
    for node in doc.descendants() {
        if !node.is_element() {
            continue;
        }
        let name = node.tag_name().name();
        let text = node.text().unwrap_or("").trim();
        if text.is_empty() {
            continue;
        }
        match name {
            "creator" => out.creator = Some(text.to_string()),
            "lastModifiedBy" => out.last_modified_by = Some(text.to_string()),
            "revision" => out.revision = Some(text.to_string()),
            _ => {}
        }
    }

    if out.creator.is_none() && out.last_modified_by.is_none() && out.revision.is_none() {
        None
    } else {
        Some(out)
    }
}
