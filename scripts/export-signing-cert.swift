// Export one code-signing identity (certificate + private key) from the
// keychain as a PKCS#12 file. Only the named identity leaves the keychain;
// `security export -t identities` would export every identity at once.
//
// Usage: P12_PASSWORD=<passphrase> swift export-signing-cert.swift <name> <out.p12>
// macOS asks for the login keychain password to allow the export.

import Foundation
import Security

func fail(_ message: String) -> Never {
    FileHandle.standardError.write((message + "\n").data(using: .utf8)!)
    exit(1)
}

let args = CommandLine.arguments
guard args.count == 3 else {
    fail("usage: P12_PASSWORD=<passphrase> swift export-signing-cert.swift <name> <out.p12>")
}
guard let passphrase = ProcessInfo.processInfo.environment["P12_PASSWORD"], !passphrase.isEmpty else {
    fail("P12_PASSWORD is not set")
}
let name = args[1]
let outPath = args[2]

let query: [String: Any] = [
    kSecClass as String: kSecClassIdentity,
    kSecAttrLabel as String: name,
    kSecMatchLimit as String: kSecMatchLimitAll,
    kSecReturnRef as String: true,
]
var found: CFTypeRef?
let status = SecItemCopyMatching(query as CFDictionary, &found)
guard status == errSecSuccess, let identities = found as? [SecIdentity] else {
    fail("no identity named '\(name)' in the keychain (status \(status))")
}
guard identities.count == 1 else {
    fail("\(identities.count) identities are named '\(name)'; expected exactly one")
}

var params = SecItemImportExportKeyParameters()
params.version = UInt32(SEC_KEY_IMPORT_EXPORT_PARAMS_VERSION)
params.passphrase = Unmanaged.passUnretained(passphrase as CFString as CFTypeRef)

var exported: CFData?
let exportStatus = SecItemExport(identities[0], .formatPKCS12, [], &params, &exported)
guard exportStatus == errSecSuccess, let data = exported else {
    fail("export of '\(name)' failed (status \(exportStatus))")
}

do {
    try (data as Data).write(to: URL(fileURLWithPath: outPath), options: .atomic)
} catch {
    fail("cannot write \(outPath): \(error)")
}
