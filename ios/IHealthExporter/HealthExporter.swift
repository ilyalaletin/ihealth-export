import Foundation
import HealthKit
import UIKit

@MainActor
final class HealthExporter: ObservableObject {
    @Published var status = "Готово"
    @Published var isRunning = false
    @Published var progress = 0.0
    @Published var sentRecords = 0
    @Published var skippedTypes = 0
    @Published var failedTypes = 0

    private let store = HKHealthStore()
    private let pageSize = 5_000
    private let iso = ISO8601DateFormatter()

    func synchronize(baseURL: String, token: String) async {
        guard !isRunning else { return }
        isRunning = true; progress = 0; sentRecords = 0; skippedTypes = 0; failedTypes = 0
        UIApplication.shared.isIdleTimerDisabled = true
        defer {
            UIApplication.shared.isIdleTimerDisabled = false
            isRunning = false
        }
        do {
            guard HKHealthStore.isHealthDataAvailable() else { throw ExportError.healthUnavailable }
            let types = HealthTypeCatalog.sampleTypes
            status = "Запрашиваю доступ к HealthKit…"
            try await store.requestAuthorization(toShare: [], read: Set(types))
            let client = APIClient(baseURL: baseURL, token: token)
            status = "Проверяю сервер…"
            try await client.health()
            let deviceID = Self.deviceID()
            let profile = readProfile()
            _ = try await client.upload(UploadBatch(deviceID: deviceID, exportedAt: iso.string(from: Date()), type: "profile", samples: [], deletedIDs: [], profile: profile))

            for (index, type) in types.enumerated() {
                status = "\(index + 1)/\(types.count): \(type.identifier)"
                do {
                    let result = try await export(type: type, deviceID: deviceID, client: client)
                    if !result.hadChanges { skippedTypes += 1 }
                } catch {
                    failedTypes += 1
                }
                progress = Double(index + 1) / Double(types.count)
            }
            if failedTypes == 0 {
                status = "Готово"
                UserDefaults.standard.set(Date(), forKey: "lastSuccessfulSync")
            } else {
                status = "Завершено с ошибками — повторите синхронизацию"
            }
        } catch {
            status = "Ошибка: \(error.localizedDescription)"
        }
    }

    private func export(type: HKSampleType, deviceID: String, client: APIClient) async throws -> (accepted: Int, hadChanges: Bool) {
        var anchor = loadAnchor(for: type.identifier)
        var accepted = 0
        var hadChanges = false
        while true {
            let page = try await anchoredPage(type: type, anchor: anchor)
            let samples = page.samples.map(encode)
            let batch = UploadBatch(deviceID: deviceID, exportedAt: iso.string(from: Date()), type: type.identifier, samples: samples, deletedIDs: page.deleted.map { $0.uuid.uuidString }, profile: nil)
            if !samples.isEmpty || !page.deleted.isEmpty {
                hadChanges = true
                let result = try await client.upload(batch)
                accepted += result.accepted
                sentRecords += result.accepted
            }
            saveAnchor(page.anchor, for: type.identifier)
            anchor = page.anchor
            if page.samples.count + page.deleted.count < pageSize { break }
        }
        return (accepted, hadChanges)
    }

    private func anchoredPage(type: HKSampleType, anchor: HKQueryAnchor?) async throws -> (samples: [HKSample], deleted: [HKDeletedObject], anchor: HKQueryAnchor) {
        try await withCheckedThrowingContinuation { continuation in
            let query = HKAnchoredObjectQuery(type: type, predicate: nil, anchor: anchor, limit: pageSize) { _, samples, deleted, newAnchor, error in
                if let error { continuation.resume(throwing: error); return }
                guard let newAnchor else { continuation.resume(throwing: ExportError.missingAnchor); return }
                continuation.resume(returning: (samples ?? [], deleted ?? [], newAnchor))
            }
            store.execute(query)
        }
    }

    private func encode(_ sample: HKSample) -> ExportSample {
        var result = ExportSample(id: sample.uuid.uuidString, type: sample.sampleType.identifier, kind: "sample", startDate: iso.string(from: sample.startDate), endDate: iso.string(from: sample.endDate), sourceName: sample.sourceRevision.source.name, sourceBundleID: sample.sourceRevision.source.bundleIdentifier, deviceName: sample.device?.name, metadata: json(sample.metadata), payload: nil)
        switch sample {
        case let quantity as HKQuantitySample:
            result.kind = "quantity"
            if let unit = CanonicalUnit.forType(sample.sampleType.identifier) {
                result.value = quantity.quantity.doubleValue(for: unit)
                result.unit = unit.unitString
            } else { result.textValue = quantity.quantity.description }
        case let category as HKCategorySample:
            result.kind = "category"; result.value = Double(category.value)
        case let workout as HKWorkout:
            result.kind = "workout"; result.activityType = Int(workout.workoutActivityType.rawValue); result.activityName = activityName(workout.workoutActivityType)
            result.value = workout.duration; result.unit = "s"
            var payload: [String: JSONValue] = ["duration": .number(workout.duration)]
            if let energyType = HKObjectType.quantityType(forIdentifier: .activeEnergyBurned),
               let energy = workout.statistics(for: energyType)?.sumQuantity() {
                payload["total_energy_kcal"] = .number(energy.doubleValue(for: .kilocalorie()))
            }
            if let distance = workout.totalDistance { payload["total_distance_m"] = .number(distance.doubleValue(for: .meter())) }
            result.payload = .object(payload)
        case let ecg as HKElectrocardiogram:
            result.kind = "electrocardiogram"; result.value = ecg.averageHeartRate?.doubleValue(for: .count().unitDivided(by: .minute())); result.unit = "count/min"
            result.payload = .object(["classification": .number(Double(ecg.classification.rawValue)), "symptoms_status": .number(Double(ecg.symptomsStatus.rawValue)), "voltage_measurements": .number(Double(ecg.numberOfVoltageMeasurements))])
        case let correlation as HKCorrelation:
            result.kind = "correlation"
            result.payload = .array(correlation.objects.map { object in .object(["id": .string(object.uuid.uuidString), "type": .string(object.sampleType.identifier)]) })
        default:
            result.kind = String(describing: Swift.type(of: sample))
            result.textValue = sample.description
        }
        return result
    }

    private func readProfile() -> ExportProfile {
        var profile = ExportProfile()
        if let components = try? store.dateOfBirthComponents(), let date = Calendar.current.date(from: components) { profile.dateOfBirth = iso.string(from: date) }
        if let value = try? store.biologicalSex() { profile.biologicalSex = String(describing: value.biologicalSex) }
        if let value = try? store.bloodType() { profile.bloodType = String(describing: value.bloodType) }
        if let value = try? store.fitzpatrickSkinType() { profile.fitzpatrickSkinType = String(describing: value.skinType) }
        if let value = try? store.wheelchairUse() { profile.wheelchairUse = String(describing: value.wheelchairUse) }
        return profile
    }

    private func json(_ dictionary: [String: Any]?) -> JSONValue? {
        guard let dictionary else { return nil }
        return .object(dictionary.mapValues { value in
            switch value { case let v as String: .string(v); case let v as NSNumber: .number(v.doubleValue); case let v as Date: .string(iso.string(from: v)); case let v as UUID: .string(v.uuidString); default: .string(String(describing: value)) }
        })
    }

    private func activityName(_ type: HKWorkoutActivityType) -> String {
        switch type {
        case .walking: "walking"; case .running: "running"; case .cycling: "cycling"; case .swimming: "swimming"; case .hiking: "hiking"; case .yoga: "yoga"; case .traditionalStrengthTraining: "traditional_strength_training"; case .functionalStrengthTraining: "functional_strength_training"; case .highIntensityIntervalTraining: "hiit"; case .coreTraining: "core_training"; case .pilates: "pilates"; case .mixedCardio: "mixed_cardio"; case .cooldown: "cooldown"; default: "activity_\(type.rawValue)"
        }
    }

    private func loadAnchor(for type: String) -> HKQueryAnchor? {
        guard let data = UserDefaults.standard.data(forKey: "anchor.\(type)") else { return nil }
        return try? NSKeyedUnarchiver.unarchivedObject(ofClass: HKQueryAnchor.self, from: data)
    }
    private func saveAnchor(_ anchor: HKQueryAnchor, for type: String) {
        if let data = try? NSKeyedArchiver.archivedData(withRootObject: anchor, requiringSecureCoding: true) { UserDefaults.standard.set(data, forKey: "anchor.\(type)") }
    }
    private static func deviceID() -> String { if let value = UserDefaults.standard.string(forKey: "deviceID") { return value }; let value = UUID().uuidString; UserDefaults.standard.set(value, forKey: "deviceID"); return value }
}

enum ExportError: LocalizedError {
    case healthUnavailable, missingAnchor
    var errorDescription: String? { switch self { case .healthUnavailable: "HealthKit недоступен"; case .missingAnchor: "HealthKit не вернул позицию синхронизации" } }
}
