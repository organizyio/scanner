use crate::limits::*;
use crate::types::{DocxMeta, PdfMeta};
use roxmltree::{Document, ParsingOptions};
use std::collections::HashSet;
use std::path::Path;

/// Best effort, bounded input/per-stream parsing and complete page-tree count.
pub fn read_pdf_info(path: &Path) -> Option<PdfMeta> {
    let bytes = read_file(path, MAX_DOCUMENT)?;
    let options = lopdf::LoadOptions::with_max_decompressed_size(MAX_PDF_STREAM);
    let doc = lopdf::Document::load_mem_with_options(&bytes, options).ok()?;
    let n = page_count(&doc)?;
    Some(PdfMeta {
        page_count: Some(n),
        author: None,
        title: None,
    })
}

fn page_count(doc: &lopdf::Document) -> Option<u32> {
    if doc.objects.len() > MAX_PDF_OBJECTS {
        return None;
    }
    let root = doc
        .catalog()
        .ok()?
        .get(b"Pages")
        .ok()?
        .as_reference()
        .ok()?;
    let mut stack = vec![(root, 0)];
    let mut seen = HashSet::new();
    let mut pages = 0u32;
    while let Some((id, depth)) = stack.pop() {
        if depth > MAX_PDF_DEPTH || !seen.insert(id) || seen.len() > MAX_PDF_OBJECTS {
            return None;
        }
        let node = doc.get_dictionary(id).ok()?;
        match node.get_type().ok()? {
            b"Page" => {
                pages = pages.checked_add(1)?;
            }
            b"Pages" => {
                let kids = node.get_deref(b"Kids", doc).ok()?.as_array().ok()?;
                if kids
                    .len()
                    .checked_add(stack.len())?
                    .checked_add(seen.len())?
                    > MAX_PDF_OBJECTS
                {
                    return None;
                }
                for child in kids.iter().rev() {
                    stack.push((child.as_reference().ok()?, depth + 1));
                }
            }
            _ => return None,
        }
    }
    (pages > 0).then_some(pages)
}

/// Read `docProps/core.xml` from a DOCX (ZIP) package.
pub fn read_docx_core(path: &Path) -> Option<DocxMeta> {
    let bytes = read_file(path, MAX_DOCUMENT)?;
    let mut archive = zip::ZipArchive::new(MetadataInput::new(&bytes, MAX_ZIP_READ)).ok()?;
    if archive.len() > MAX_ZIP_ENTRIES {
        return None;
    }
    let zf = archive.by_name("docProps/core.xml").ok()?;
    if zf.size() > MAX_CORE_XML {
        return None;
    }
    let xml = String::from_utf8(read(zf, MAX_CORE_XML)?).ok()?;
    let doc = Document::parse_with_options(
        &xml,
        ParsingOptions {
            allow_dtd: false,
            nodes_limit: MAX_XML_NODES,
        },
    )
    .ok()?;

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

#[cfg(test)]
mod resource_tests {
    use super::*;
    use std::io::Write;
    use zip::{write::SimpleFileOptions, CompressionMethod, ZipWriter};
    fn xml(length: usize) -> Vec<u8> {
        let head="<cp:coreProperties xmlns:cp='urn:cp' xmlns:dc='urn:dc'><dc:creator>Synthetic é🙂</dc:creator><cp:revision>007</cp:revision><dc:description>";
        let tail = "</dc:description></cp:coreProperties>";
        [
            head.as_bytes(),
            &vec![b'x'; length - head.len() - tail.len()],
            tail.as_bytes(),
        ]
        .concat()
    }
    fn archive(body: &[u8], method: CompressionMethod, extras: usize) -> tempfile::NamedTempFile {
        let file = tempfile::NamedTempFile::new().unwrap();
        let mut z = ZipWriter::new(file.reopen().unwrap());
        z.start_file(
            "docProps/core.xml",
            SimpleFileOptions::default()
                .compression_method(method)
                .large_file(true),
        )
        .unwrap();
        z.write_all(body).unwrap();
        for i in 0..extras {
            z.start_file(format!("unused-{i}"), SimpleFileOptions::default())
                .unwrap();
        }
        z.finish().unwrap();
        file
    }
    #[test]
    fn docx_expansion_limit_is_inclusive_and_preserves_utf8_fields() {
        for method in [CompressionMethod::Stored, CompressionMethod::Deflated] {
            let f = archive(&xml(MAX_CORE_XML as usize), method, 0);
            let m = read_docx_core(f.path()).unwrap();
            assert_eq!(m.creator.as_deref(), Some("Synthetic é🙂"));
            assert_eq!(m.revision.as_deref(), Some("007"));
            let large = archive(&xml(MAX_CORE_XML as usize + 1), method, 0);
            assert!(read_docx_core(large.path()).is_none());
            assert!(read_docx_core(f.path()).is_some());
        }
    }
    #[test]
    fn xml_nodes_and_archive_entries_have_independent_budgets() {
        let dense = format!(
            "<root><creator>Synthetic</creator>{}</root>",
            "<x/>".repeat(MAX_XML_NODES as usize)
        );
        let f = archive(dense.as_bytes(), CompressionMethod::Deflated, 0);
        assert!(read_docx_core(f.path()).is_none());
        let f = archive(&xml(256), CompressionMethod::Deflated, MAX_ZIP_ENTRIES - 1);
        assert!(read_docx_core(f.path()).is_some());
        let f = archive(&xml(256), CompressionMethod::Deflated, MAX_ZIP_ENTRIES);
        assert!(read_docx_core(f.path()).is_none());
    }
    #[test]
    fn advertised_xml_size_cannot_authorize_oversized_actual_expansion() {
        let f = archive(
            &xml(MAX_CORE_XML as usize + 1),
            CompressionMethod::Deflated,
            0,
        );
        let mut bytes = std::fs::read(f.path()).unwrap();
        let cd = bytes.windows(4).position(|v| v == b"PK\x01\x02").unwrap();
        bytes[cd + 24..cd + 28].copy_from_slice(&256u32.to_le_bytes());
        std::fs::write(f.path(), bytes).unwrap();
        assert!(read_docx_core(f.path()).is_none());
    }
    fn page_document() -> lopdf::Document {
        use lopdf::{dictionary, Object};
        let mut d = lopdf::Document::with_version("1.5");
        d.objects.insert(
            (1, 0),
            Object::Dictionary(dictionary! {"Type"=>"Catalog","Pages"=>(2,0)}),
        );
        d.objects.insert(
            (2, 0),
            Object::Dictionary(
                dictionary! {"Type"=>"Pages","Kids"=>vec![Object::Reference((3,0))],"Count"=>1},
            ),
        );
        d.objects.insert((3,0),Object::Dictionary(dictionary!{"Type"=>"Page","Parent"=>(2,0),"MediaBox"=>vec![0.into(),0.into(),100.into(),100.into()]}));
        d.max_id = 3;
        d.trailer.set("Root", (1, 0));
        d
    }
    #[test]
    fn page_tree_cycles_duplicate_references_and_missing_children_are_unknown() {
        use lopdf::Object;
        for kids in [
            vec![Object::Reference((2, 0))],
            vec![Object::Reference((3, 0)), Object::Reference((3, 0))],
            vec![Object::Reference((3, 0)), Object::Reference((900, 0))],
        ] {
            let mut d = page_document();
            d.get_object_mut((2, 0))
                .unwrap()
                .as_dict_mut()
                .unwrap()
                .set("Kids", kids);
            assert!(page_count(&d).is_none());
        }
        let mut d = page_document();
        d.get_object_mut((2, 0))
            .unwrap()
            .as_dict_mut()
            .unwrap()
            .set("Count", i64::MAX);
        assert_eq!(page_count(&d), Some(1));
        for i in 4..=MAX_PDF_OBJECTS + 1 {
            d.objects.insert((i as u32, 0), Object::Null);
        }
        assert!(page_count(&d).is_none());
    }
    #[test]
    fn compressed_object_stream_limit_is_exercised_with_valid_pdf_controls() {
        use lopdf::Object;
        for padding in [4096, MAX_PDF_STREAM + 1] {
            let mut d = page_document();
            let pad = d.add_object(Object::string_literal(vec![b'x'; padding]));
            d.get_object_mut((1, 0))
                .unwrap()
                .as_dict_mut()
                .unwrap()
                .set("SyntheticPad", pad);
            d.trailer.set(
                "ID",
                vec![Object::string_literal(b"synthetic-pdf-id".to_vec()); 2],
            );
            let f = tempfile::NamedTempFile::new().unwrap();
            d.save_modern(&mut f.reopen().unwrap()).unwrap();
            let raw = std::fs::read(f.path()).unwrap();
            let baseline = lopdf::Document::load_mem(&raw).unwrap();
            assert_eq!(baseline.get_pages().len(), 1);
            if padding == 4096 {
                assert_eq!(read_pdf_info(f.path()).unwrap().page_count, Some(1));
            } else {
                assert!(read_pdf_info(f.path()).is_none());
            }
        }
    }
}
