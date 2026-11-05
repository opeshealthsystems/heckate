import { useCallback, useEffect, useState } from "react";
import { Image, ScrollView, View } from "react-native";
import { api, attachmentUrl, authHeader, uploadPhoto } from "../lib/api";
import { chooseAction } from "../lib/dialogs";
import { haptic } from "../lib/haptics";
import { pickPhoto, type Photo } from "../lib/photos";
import { radius, sp, useTheme } from "../lib/theme";
import type { Attachment } from "../lib/types";
import { Row, Section } from "./List";

/** A row that opens the native action sheet (Take photo / Choose photo) and hands the picked photo to `onPhoto`. */
export function AddPhotoRow({ onPhoto, onError, subtitle, disabled }: { onPhoto: (p: Photo) => void | Promise<void>; onError: (m: string) => void; subtitle?: string; disabled?: boolean }) {
  const pick = (source: "camera" | "library") => void pickPhoto(source).then((p) => (p ? onPhoto(p) : undefined)).catch((e: Error) => onError(e.message));
  return (
    <Row
      icon="camera"
      action
      title="Add photo"
      subtitle={subtitle}
      disabled={disabled}
      onPress={() => chooseAction("Add photo", [{ label: "Take photo", onPress: () => pick("camera") }, { label: "Choose photo", onPress: () => pick("library") }])}
    />
  );
}

/** Add-photo row that uploads straight to a record and says so. */
export function PhotoUploadRow({ entityType, entityId, onUploaded }: { entityType: "ticket" | "intervention" | "expense"; entityId: string; onUploaded?: () => void }) {
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  return (
    <AddPhotoRow
      disabled={busy}
      subtitle={busy ? "Uploading…" : msg || undefined}
      onError={(m) => { haptic.error(); setMsg(m); }}
      onPhoto={async (p) => {
        setBusy(true);
        setMsg("");
        try {
          await uploadPhoto(entityType, entityId, p);
          haptic.success();
          setMsg("Photo attached.");
          onUploaded?.();
        } catch (e) {
          haptic.error();
          setMsg((e as Error).message);
        }
        setBusy(false);
      }}
    />
  );
}

/** Thumbnails of image attachments (fetched with the bearer token) and rows for other files. Nothing when there are none. */
export function AttachmentsSection({ entityType, entityId, refreshKey, children }: { entityType: string; entityId: string; refreshKey: number; children?: React.ReactNode }) {
  const { c } = useTheme();
  const [files, setFiles] = useState<Attachment[]>([]);
  const load = useCallback(() => api.attachments(entityType, entityId).then((r) => setFiles(r.data)).catch(() => {}), [entityType, entityId]);
  useEffect(() => { void load(); }, [load, refreshKey]);
  const images = files.filter((a) => a.content_type.startsWith("image/"));
  const others = files.filter((a) => !a.content_type.startsWith("image/"));
  return (
    <Section header={files.length ? `Photos and files · ${files.length}` : "Photos"}>
      {images.length > 0 && (
        <View>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={{ padding: sp.lg, gap: sp.sm }}>
            {images.map((a) => (
              <Image key={a.id} accessibilityLabel={a.filename} source={{ uri: attachmentUrl(a.id), headers: authHeader() }} style={{ width: 88, height: 88, borderRadius: radius.field, backgroundColor: c.bg }} />
            ))}
          </ScrollView>
        </View>
      )}
      {others.map((a) => <Row key={a.id} icon="documents" title={a.filename} iconTone="label2" />)}
      {children}
    </Section>
  );
}
