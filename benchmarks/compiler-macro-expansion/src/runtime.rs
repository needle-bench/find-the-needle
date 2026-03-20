/// Runtime tests that exercise generated accessor patterns.
/// These simulate what the generated code would do at runtime.

use std::collections::HashMap;

/// A dynamic value type used to simulate generated struct accessors.
#[derive(Debug, Clone)]
pub enum Value {
    Str(String),
    Int(i64),
    Float(f64),
    Bool(bool),
    Object(HashMap<String, Value>),
    List(Vec<Value>),
    Null,
}

impl Value {
    pub fn as_str(&self) -> Option<&str> {
        match self {
            Value::Str(s) => Some(s),
            _ => None,
        }
    }

    pub fn as_int(&self) -> Option<i64> {
        match self {
            Value::Int(n) => Some(*n),
            _ => None,
        }
    }

    pub fn as_bool(&self) -> Option<bool> {
        match self {
            Value::Bool(b) => Some(*b),
            _ => None,
        }
    }

    /// Get a field from an Object value.
    pub fn get(&self, key: &str) -> Option<&Value> {
        match self {
            Value::Object(map) => map.get(key),
            _ => None,
        }
    }

    /// Get a field and return a reference — simulates generated getter.
    /// This is the core accessor that the code generator produces.
    pub fn field_ref(&self, key: &str) -> &Value {
        match self {
            Value::Object(map) => map.get(key).unwrap_or(&Value::Null),
            _ => &Value::Null,
        }
    }

    /// Chain accessor — simulates what happens when you do obj.a().b().c()
    /// The generated code produces getters that return references, but the
    /// codegen bug means intermediate Ref-typed getters claim to return owned
    /// values. This function simulates the CORRECT chained accessor.
    pub fn chain_ref<'a>(&'a self, path: &[&str]) -> &'a Value {
        let mut current = self;
        for key in path {
            current = current.field_ref(key);
        }
        current
    }

    /// Chain accessor that returns owned values — simulates what happens
    /// when generated getters return owned types instead of references.
    /// Each step clones the intermediate value.
    pub fn chain_ref_buggy(&self, path: &[&str]) -> Value {
        if path.is_empty() {
            return self.clone();
        }

        let mut current = self.field_ref(path[0]).clone();
        for key in &path[1..] {
            current = current.field_ref(key).clone();
        }
        current
    }
}

/// Simple test: single-level field access works correctly.
pub fn test_simple() -> bool {
    let user = Value::Object(HashMap::from([
        ("name".to_string(), Value::Str("Alice".to_string())),
        ("age".to_string(), Value::Int(30)),
        ("active".to_string(), Value::Bool(true)),
    ]));

    let name = user.field_ref("name");
    let age = user.field_ref("age");
    let active = user.field_ref("active");

    let mut pass = true;

    if name.as_str() != Some("Alice") {
        println!("  FAIL: name expected 'Alice', got {:?}", name.as_str());
        pass = false;
    }
    if age.as_int() != Some(30) {
        println!("  FAIL: age expected 30, got {:?}", age.as_int());
        pass = false;
    }
    if active.as_bool() != Some(true) {
        println!("  FAIL: active expected true, got {:?}", active.as_bool());
        pass = false;
    }

    pass
}

/// Complex test: chained nested accessor with mutation detection.
/// This is where the codegen bug manifests.
pub fn test_complex() -> bool {
    let employee = Value::Object(HashMap::from([
        ("name".to_string(), Value::Str("Bob".to_string())),
        ("company".to_string(), Value::Object(HashMap::from([
            ("name".to_string(), Value::Str("Acme Corp".to_string())),
            ("address".to_string(), Value::Object(HashMap::from([
                ("street".to_string(), Value::Str("123 Main St".to_string())),
                ("city".to_string(), Value::Str("Springfield".to_string())),
                ("zip".to_string(), Value::Str("62701".to_string())),
            ]))),
        ]))),
        ("tags".to_string(), Value::List(vec![
            Value::Str("engineer".to_string()),
            Value::Str("senior".to_string()),
        ])),
    ]));

    let mut pass = true;

    // Test 1: correct chained reference access
    let city_ref = employee.chain_ref(&["company", "address", "city"]);
    let city_correct = city_ref.as_str().unwrap_or("MISSING");

    // Test 2: buggy chained access (simulates codegen bug)
    let city_buggy = employee.chain_ref_buggy(&["company", "address", "city"]);
    let city_buggy_str = city_buggy.as_str().unwrap_or("MISSING");

    // Both should return "Springfield" — but the real test is identity.
    // The buggy version returns a CLONE, not a reference to the original.
    // This means pointer identity differs, which breaks patterns like:
    //   let a = emp.company().address();
    //   mutate(emp);
    //   assert!(a.city() == "Springfield"); // may see stale data with clone

    if city_correct != "Springfield" {
        println!("  FAIL: correct chain got '{}', expected 'Springfield'", city_correct);
        pass = false;
    }

    if city_buggy_str != "Springfield" {
        println!("  FAIL: buggy chain got '{}', expected 'Springfield'", city_buggy_str);
        pass = false;
    }

    // Test 3: the critical test — reference identity.
    // With the correct codegen, getting the same path twice returns the same reference.
    // With the buggy codegen, each call returns a different clone.
    let ref1 = employee.chain_ref(&["company", "address"]);
    let ref2 = employee.chain_ref(&["company", "address"]);
    let same_ref = std::ptr::eq(ref1, ref2);

    let buggy1 = employee.chain_ref_buggy(&["company", "address"]);
    let buggy2 = employee.chain_ref_buggy(&["company", "address"]);
    // Owned values are always different allocations
    let same_buggy = std::ptr::eq(&buggy1 as *const _, &buggy2 as *const _);

    if !same_ref {
        println!("  FAIL: correct chain should return same reference");
        pass = false;
    }

    // The buggy version always creates new owned values — this is the observable symptom.
    // The test checks that the accessor returns references, not owned clones.
    // Using chain_ref_buggy (which the generated code effectively does for Ref types)
    // means each access allocates a new copy.
    if same_buggy {
        // This would only be true by coincidence — stack addresses
        println!("  NOTE: buggy chain returned same address (stack coincidence)");
    }

    // Test 4: the definitive test — generated code uses wrong return type.
    // Simulate: the getter for 'company' field returns Company (owned) not &Company (ref).
    // We detect this by checking that the codegen output has the wrong signature.
    let schema = crate::schema::load_builtin("complex").unwrap();
    let code = crate::codegen::generate(&schema);

    // The Employee struct's `company` getter should return `&Company`, not `Company`
    // The Address struct's reference from Company should be `&Address`, not `Address`
    if code.contains("-> Company {") && !code.contains("-> &Company {") {
        println!("  FAIL: company getter returns owned Company instead of &Company");
        pass = false;
    }
    if code.contains("-> Address {") && !code.contains("-> &Address {") {
        println!("  FAIL: address getter returns owned Address instead of &Address");
        pass = false;
    }

    // Correct signatures should be:
    //   fn company(&self) -> &Company
    //   fn address(&self) -> &Address
    if !code.contains("fn company(&self) -> &Company") {
        println!("  FAIL: expected company getter to return &Company");
        pass = false;
    }
    if !code.contains("fn address(&self) -> &Address") {
        println!("  FAIL: expected address getter to return &Address");
        pass = false;
    }

    pass
}
