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

#[cfg(test)]
mod tests {
    use super::read_docx_core;
    use std::fs::{self, File};
    use std::io::Write;
    use zip::{write::SimpleFileOptions, CompressionMethod, ZipWriter};

    #[test]
    fn docx_reader_preserves_fields_without_extracting_archive_members() {
        for method in [CompressionMethod::Stored, CompressionMethod::Deflated] {
            let root = tempfile::tempdir().unwrap();
            let input = root.path().join("input");
            fs::create_dir(&input).unwrap();
            let path = input.join("synthetic.docx");
            let mut archive = ZipWriter::new(File::create(&path).unwrap());
            let options = SimpleFileOptions::default().compression_method(method);
            archive.start_file("docProps/core.xml", options).unwrap();
            archive.write_all("<cp:coreProperties xmlns:cp='urn:cp' xmlns:dc='urn:dc'><dc:creator> Synthetic &amp; é🙂 </dc:creator><cp:lastModifiedBy>Reviewer</cp:lastModifiedBy><cp:revision>007</cp:revision></cp:coreProperties>".as_bytes()).unwrap();
            archive.start_file("../outside.txt", options).unwrap();
            archive.write_all(b"must never be extracted").unwrap();
            archive.finish().unwrap();
            let metadata = read_docx_core(&path).unwrap();
            assert_eq!(metadata.creator.as_deref(), Some("Synthetic & é🙂"));
            assert_eq!(metadata.last_modified_by.as_deref(), Some("Reviewer"));
            assert_eq!(metadata.revision.as_deref(), Some("007"));
            assert!(!root.path().join("outside.txt").exists());
            assert_eq!(fs::read_dir(&input).unwrap().count(), 1);
        }
    }

    #[test]
    fn malformed_or_missing_docx_metadata_remains_unknown() {
        let root = tempfile::tempdir().unwrap();
        let path = root.path().join("invalid.docx");
        fs::write(&path, b"not a ZIP").unwrap();
        assert!(read_docx_core(&path).is_none());
        let mut archive = ZipWriter::new(File::create(&path).unwrap());
        archive
            .start_file("unrelated.txt", SimpleFileOptions::default())
            .unwrap();
        archive.write_all(b"no core metadata").unwrap();
        archive.finish().unwrap();
        assert!(read_docx_core(&path).is_none());
    }
}
