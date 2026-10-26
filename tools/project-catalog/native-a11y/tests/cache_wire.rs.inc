#[cfg(all(test, not(feature = "tokio")))]
mod tests {
    use super::*;
    use atspi::CacheItem;
    use futures_lite::StreamExt;
    use zbus::{MatchRule, MessageStream, message::Type};

    fn assert_wire_signature(message: &zbus::Message, signature: &str) {
        let bytes = message.data().bytes();
        let length = bytes[12..16].try_into().unwrap();
        let length = match bytes[0] {
            b'l' => u32::from_le_bytes(length),
            b'B' => u32::from_be_bytes(length),
            _ => panic!("invalid D-Bus byte order"),
        } as usize;
        // Inspect the encoded header: zbus normalizes outer parentheses when reading it.
        let mut field = vec![8, 1, b'g', 0, signature.len() as u8];
        field.extend_from_slice(signature.as_bytes());
        field.push(0);
        assert!(
            bytes[16..16 + length]
                .windows(field.len())
                .any(|w| w == field)
        );
    }

    #[test]
    #[ignore = "requires a private D-Bus session; run with dbus-run-session"]
    fn cache_signals_preserve_struct_arguments() {
        futures_lite::future::block_on(async {
            let conn = Connection::session().await.unwrap();
            let rule = MatchRule::builder()
                .msg_type(Type::Signal)
                .interface("org.a11y.atspi.Cache")
                .unwrap()
                .path(cache_path())
                .unwrap()
                .build();
            let mut signals = MessageStream::for_match_rule(rule, &conn, Some(2))
                .await
                .unwrap();
            let executor = Executor::new();
            let bus = Bus {
                conn: conn.clone(),
                _task: executor.spawn(std::future::pending(), "unused"),
                socket_proxy: SocketProxy::new(&conn).await.unwrap(),
                desktop: Arc::new(OnceLock::new()),
            };
            let item = CacheItem::default();
            bus.emit_cache_signal("AddAccessible", &item).await.unwrap();
            let added = signals.next().await.unwrap().unwrap();
            assert_eq!(added.header().member().unwrap().as_str(), "AddAccessible");
            assert_wire_signature(&added, "((so)(so)(so)iiassusau)");
            let (received,): (CacheItem,) = added.body().deserialize().unwrap();
            assert_eq!(received, item);

            bus.emit_cache_signal("RemoveAccessible", &item.object)
                .await
                .unwrap();
            let removed = signals.next().await.unwrap().unwrap();
            assert_eq!(
                removed.header().member().unwrap().as_str(),
                "RemoveAccessible"
            );
            assert_wire_signature(&removed, "(so)");
            let (received,): (ObjectRefOwned,) = removed.body().deserialize().unwrap();
            assert_eq!(received, item.object);
        });
    }
}
