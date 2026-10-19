import { useCallback, useEffect, useState } from "react";
import { Image, Text, View } from "react-native";
import { api, attachmentUrl, authHeader } from "../lib/api";
import { useTheme } from "../lib/theme";
import type { Attachment } from "../lib/types";

/** Thumbnails of image attachments (fetched with the bearer token) and names of other files. */
export function AttachmentList({ entityType, entityId, refreshKey }: { entityType: string; entityId: string; refreshKey: number }) {
  const { s } = useTheme();
  const [files, setFiles] = useState<Attachment[]>([]);
  const load = useCallback(() => api.attachments(entityType, entityId).then((r) => setFiles(r.data)).catch(() => {}), [entityType, entityId]);
  useEffect(() => { void load(); }, [load, refreshKey]);
  if (files.length === 0) return null;
  return (
    <View style={{ gap: 8 }}>
      <Text style={s.label}>Attachments</Text>
      <View style={s.row}>
        {files.map((a) =>
          a.content_type.startsWith("image/") ? (
            <Image key={a.id} accessibilityLabel={a.filename} source={{ uri: attachmentUrl(a.id), headers: authHeader() }} style={{ width: 96, height: 96, borderRadius: 8 }} />
          ) : (
            <View key={a.id} style={s.card}><Text style={s.muted}>{a.filename}</Text></View>
          ),
        )}
      </View>
    </View>
  );
}
