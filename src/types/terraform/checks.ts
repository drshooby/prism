/**
 * Checks Representation — A property of both the plan and state representations that describes the current status of any checks (e.g. preconditions and postconditions) in the configuration.
 *
 * A <checks-representation> describes the current state of a checkable object in the configuration. For example, a resource with one or more preconditions or postconditions is an example of a checkable object, and its check state represents the results of those conditions.
 */
export interface Check {
  /**
   * Describes the address of the checkable object whose status this object is describing.
   */
  address: Address

  /**
   * The aggregate status of all of the instances of the object being described by this object. The possible values are "pass", "fail", "error", and "unknown".
   */
  status: "pass" | "fail" | "error" | "unknown"

  /**
   * "instances" describes the current status of each of the instances of the object being described. An object can have multiple instances if it is either a resource which has "count" or "for_each" set, or if it's contained within a module that has "count" or "for_each" set.
   *
   * If "instances" is empty or omitted, that can either mean that the object has no instances at all (e.g. count = 0) or that an error blocked evaluation of the repetition argument. You can distinguish these cases using the "status" property, which will be "pass" or "error" for a zero-instance object and "unknown" for situations where an error blocked evalation.
   */
  instances?: CheckInstance[]
}

/** Address object for checks. */
export interface Address {
  /**
   * Specifies what kind of checkable object this is. Different kinds of object will have different additional properties inside the address object, but all kinds include both "kind" and "to_display". The two valid kinds are "resource" and "output_value".
   */
  kind: "resource" | "output_value"

  /**
   * "to_display" contains an opaque string representation of the address of the object that is suitable for display in a UI. For consumers that have special handling depending on the value of "kind", this property is a good fallback to use when the application doesn't recognize the "kind" value.
   */
  to_display: string

  /**
   * Included for kind "resource" only, and specifies the resource mode which can either be "managed" (for "resource" blocks) or "data" (for "data" blocks).
   */
  mode?: "managed" | "data"

  /** Included for kind "resource" only */
  type?: string

  /**
   * "name" is the local name of the object. For a resource this is the second label in the resource block header, and for an output value this is the single label in the output block header.
   */
  name: string

  /**
   * "module" is included if the object belongs to a module other than the root module, and provides an opaque string representation of the module this object belongs to. This example is of a root module resource and so "module" is not included.
   */
  module?: string
}

/** Per-instance status inside a Check. */
export interface CheckInstance {
  /**
   * "address" is an object similar to the property of the same name in the containing object. Merge the instance-level address into the object-level address, overwriting any conflicting property names, to create a full description of the instance's address.
   */
  address: {
    /**
     * "to_display" overrides the property of the same name in the main object's address, to include any module instance or resource instance keys that uniquely identify this instance.
     */
    to_display: string

    /**
     * "instance_key" is included for resources only and specifies the resource-level instance key, which can either be a number or a string. Omitted for single-instance resources.
     */
    instance_key?: number | string

    /**
     * "module" is included if the object belongs to a module other than the root module, and provides an opaque string representation of the module instance this object belongs to.
     */
    module?: string
  }

  /**
   * "status" describes the result of running the configured checks against this particular instance of the object, with the same possible values as the "status" in the parent object.
   *
   * "fail" means that the condition evaluated successfully but returned false, while "error" means that the condition expression itself was invalid.
   */
  status: "pass" | "fail" | "error" | "unknown"

  /**
   * "problems" might be included for statuses "fail" or "error", in which case it describes the individual conditions that failed for this instance, if any.
   *
   * When a condition expression is invalid, Terraform returns that as a normal error message rather than as a problem in this list.
   */
  problems?: {
    /**
     * "message" is the string that resulted from evaluating the error_message argument of the failing condition.
     */
    message: string[]
  }
}
