import { APIKeyRecord } from "./api";

export type StoredKey = {
  id: string;
  name: string;
  keyPrefix: string;
  rawKey: string;
};

export type SelectableKey = APIKeyRecord & {
  rawKey: string | null;
};

export function mergeSelectableKeys(apiKeys: APIKeyRecord[], storedKeys: StoredKey[]): SelectableKey[] {
  const rawKeysByID = new Map(storedKeys.map((item) => [item.id, item.rawKey]));

  return apiKeys
    .filter((item) => item.is_active)
    .map((item) => ({
      ...item,
      rawKey: rawKeysByID.get(item.id) ?? null,
    }));
}
