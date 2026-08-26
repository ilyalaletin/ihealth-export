import Foundation

struct ExportSample: Codable {
    let id: String
    let type: String
    var kind: String
    let startDate: String
    let endDate: String
    var value: Double?
    var textValue: String?
    var unit: String?
    var activityType: Int?
    var activityName: String?
    var sourceName: String?
    var sourceBundleID: String?
    var deviceName: String?
    var metadata: JSONValue?
    var payload: JSONValue?

    enum CodingKeys: String, CodingKey {
        case id, type, kind, value, unit, metadata, payload
        case startDate = "start_date"
        case endDate = "end_date"
        case textValue = "text_value"
        case activityType = "activity_type"
        case activityName = "activity_name"
        case sourceName = "source_name"
        case sourceBundleID = "source_bundle_id"
        case deviceName = "device_name"
    }
}

struct ExportProfile: Codable {
    var dateOfBirth: String?
    var biologicalSex: String?
    var bloodType: String?
    var fitzpatrickSkinType: String?
    var wheelchairUse: String?

    enum CodingKeys: String, CodingKey {
        case dateOfBirth = "date_of_birth"
        case biologicalSex = "biological_sex"
        case bloodType = "blood_type"
        case fitzpatrickSkinType = "fitzpatrick_skin_type"
        case wheelchairUse = "wheelchair_use"
    }
}

struct UploadBatch: Codable {
    let deviceID: String
    let exportedAt: String
    let type: String
    let samples: [ExportSample]
    let deletedIDs: [String]
    var profile: ExportProfile?

    enum CodingKeys: String, CodingKey {
        case type, samples, profile
        case deviceID = "device_id"
        case exportedAt = "exported_at"
        case deletedIDs = "deleted_ids"
    }
}

struct UploadResult: Codable { let accepted: Int; let deleted: Int }

enum JSONValue: Codable {
    case string(String), number(Double), bool(Bool), object([String: JSONValue]), array([JSONValue]), null

    init(from decoder: Decoder) throws {
        let box = try decoder.singleValueContainer()
        if box.decodeNil() { self = .null }
        else if let value = try? box.decode(Bool.self) { self = .bool(value) }
        else if let value = try? box.decode(Double.self) { self = .number(value) }
        else if let value = try? box.decode(String.self) { self = .string(value) }
        else if let value = try? box.decode([String: JSONValue].self) { self = .object(value) }
        else { self = .array(try box.decode([JSONValue].self)) }
    }

    func encode(to encoder: Encoder) throws {
        var box = encoder.singleValueContainer()
        switch self {
        case .string(let value): try box.encode(value)
        case .number(let value): try box.encode(value)
        case .bool(let value): try box.encode(value)
        case .object(let value): try box.encode(value)
        case .array(let value): try box.encode(value)
        case .null: try box.encodeNil()
        }
    }
}
