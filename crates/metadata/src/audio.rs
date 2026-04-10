use crate::types::AudioMeta;
use lofty::file::TaggedFileExt;
use lofty::probe::Probe;
use lofty::tag::Accessor;
use std::path::Path;

pub fn read_audio_tags(path: &Path) -> Option<AudioMeta> {
    // Extension-based `FileType` from `Probe::open` is enough for normal paths.
    let tagged = Probe::open(path).ok()?.read().ok()?;
    let tag = tagged.primary_tag().or_else(|| tagged.first_tag())?;

    let m = AudioMeta {
        artist: tag.artist().map(|s| s.to_string()),
        title: tag.title().map(|s| s.to_string()),
        album: tag.album().map(|s| s.to_string()),
    };

    if m.artist.is_none() && m.title.is_none() && m.album.is_none() {
        None
    } else {
        Some(m)
    }
}
