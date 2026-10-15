use criterion::{black_box, criterion_group, criterion_main, Criterion};
use engine::{scan, ScanOptions};
use std::io;
use tempfile::tempdir;
use walker::FilterOptions;

fn walk_only(c: &mut Criterion) {
    let dir = tempdir().unwrap();
    for i in 0..512u32 {
        std::fs::write(dir.path().join(format!("f{i}.txt")), b"x").unwrap();
    }
    let root = dir.path().to_path_buf();
    let opts = ScanOptions {
        roots: vec![root],
        filter: FilterOptions::default(),
        ..Default::default()
    };
    c.bench_function("walk_512_files", |b| {
        b.iter(|| {
            let mut sink = io::sink();
            scan(black_box(&opts), black_box(&mut sink)).unwrap();
        });
    });
}

criterion_group!(benches, walk_only);
criterion_main!(benches);
