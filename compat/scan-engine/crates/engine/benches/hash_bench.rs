use criterion::{black_box, criterion_group, criterion_main, Criterion};
use hash::{hash_file_sha256_hex, hash_file_xxhash64_hex};
use tempfile::NamedTempFile;

fn hash_throughput(c: &mut Criterion) {
    let mut f = NamedTempFile::new().unwrap();
    let data = vec![0xabu8; 256 * 1024];
    std::io::Write::write_all(&mut f, &data).unwrap();
    let path = f.path().to_path_buf();

    c.bench_function("xxhash_256kib", |b| {
        b.iter(|| {
            hash_file_xxhash64_hex(black_box(&path)).unwrap();
        });
    });

    c.bench_function("sha256_256kib", |b| {
        b.iter(|| {
            hash_file_sha256_hex(black_box(&path)).unwrap();
        });
    });
}

criterion_group!(benches, hash_throughput);
criterion_main!(benches);
