import type { Values } from "./values";

/**
 * Change Representation — A sub-object of plan output that describes changes to an object.
 */
export interface Change {
  /**
   * The actions that will be taken on the object selected by the properties below.
   * Valid actions values are:
   *
   *   - ["no-op"]
   *   - ["create"]
   *   - ["read"]
   *   - ["update"]
   *   - ["delete", "create"]
   *   - ["create", "delete"]
   *   - ["delete"]
   *
   * The two "replace" actions are represented in this way to allow callers to e.g. just scan the list for "delete" to recognize all three situations where the object will be deleted, allowing for any new deletion combinations that might be added in future.
   */
  actions:
    | ["no-op"]
    | ["create"]
    | ["read"]
    | ["update"]
    | ["delete", "create"]
    | ["create", "delete"]
    | ["delete"];

  /**
   * "before" and "after" are representations of the object value both before and after the action. For ["create"] and ["delete"] actions, either "before" or "after" is unset (respectively). For ["no-op"], the before and after values are identical. The "after" value will be incomplete if there are values within it that won't be known until after apply.
   */
  before: Values | null;

  /**
   * Representation of the object after the change. Unset for ["delete"].
   * May be incomplete (some leaves may be omitted if unknown).
   */
  after: Values | null;

  /**
   * an object value with similar structure to "after", but with all unknown leaf values replaced with "true", and all known leaf values omitted. This can be combined with "after" to reconstruct a full value after the action, including values which will only be known after apply.
   */
  after_unknown: Values | null;

  /**
   * "before_sensitive" and "after_sensitive" are object values with similar structure to "before" and "after", but with all sensitive leaf values replaced with true, and all non-sensitive leaf values omitted. These objects should be combined with "before" and "after" to prevent accidental display of sensitive values in user interfaces.
   */
  before_sensitive: Values | null;
  after_sensitive: Values | null;

  /**
   * An array of arrays representing a set of paths into the object value which resulted in the action being "replace". This will be omitted if the action is not replace, or if no paths caused the replacement (for example, if the resource was tainted). Each path consists of one or more steps, each of which will be a number or a string.
   *
   * Example: [["triggers"]]
   */
  replace_paths?: Array<Array<number | string>>;

  /**
   * Present only when the object is being imported as part of this change.
   *
   * Example: { "id": "foo" }
   */
  importing?: { id: string };
}
