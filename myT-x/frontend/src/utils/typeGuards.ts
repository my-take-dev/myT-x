/**
 * Returns payload when it is a non-array object.
 * This helper does not validate the structure of T.
 * Callers must validate required properties before use.
 */
export function asObject<T extends object>(payload: unknown): T | null {
  if (payload == null || typeof payload !== "object" || Array.isArray(payload)) {
    return null;
  }
  return payload as T;
}

/**
 * Returns payload when it is an array.
 * This helper does not validate element types.
 * Callers must validate array element structure before use.
 */
export function asArray<T>(payload: unknown): T[] | null {
  if (!Array.isArray(payload)) {
    return null;
  }
  return payload as T[];
}
